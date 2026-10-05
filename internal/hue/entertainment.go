package hue

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mathisarends/huego/internal/config"
	"github.com/pion/dtls/v3"
)

type EntertainmentFrame map[string][]float64

func EncodeEntertainmentFrame(areaID string, sequence byte, colors EntertainmentFrame) ([]byte, error) {
	if !IsUUID(areaID) {
		return nil, fmt.Errorf("entertainment area must be a UUID")
	}
	if len(colors) == 0 || len(colors) > 20 {
		return nil, fmt.Errorf("a frame must contain 1-20 channels")
	}
	channels := make([]int, 0, len(colors))
	for key, color := range colors {
		id, err := strconv.Atoi(key)
		if err != nil || id < 0 || id > 255 || strconv.Itoa(id) != key {
			return nil, fmt.Errorf("invalid channel ID %q", key)
		}
		if len(color) != 3 {
			return nil, fmt.Errorf("channel %s requires three RGB components", key)
		}
		for _, value := range color {
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
				return nil, fmt.Errorf("RGB components must be finite values between 0 and 1")
			}
		}
		channels = append(channels, id)
	}
	sort.Ints(channels)
	frame := append([]byte("HueStream"), 2, 0, sequence, 0, 0, 0, 0)
	frame = append(frame, strings.ToLower(areaID)...)
	for _, id := range channels {
		frame = append(frame, byte(id))
		for _, value := range colors[strconv.Itoa(id)] {
			frame = binary.BigEndian.AppendUint16(frame, uint16(math.Round(value*65535)))
		}
	}
	return frame, nil
}

var dialEntertainment = func(ctx context.Context, bridge config.Bridge) (io.WriteCloser, error) {
	key, err := hex.DecodeString(bridge.ClientKey)
	if err != nil || len(key) != 16 {
		return nil, fmt.Errorf("entertainment requires the 32-character hexadecimal client key from the same pairing as the app key")
	}
	host := bridge.IP
	if value, _, err := net.SplitHostPort(host); err == nil {
		host = value
	}
	address, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, "2100"))
	if err != nil {
		return nil, err
	}
	return connectEntertainment(ctx, address, bridge.AppKey, key)
}

func connectEntertainment(ctx context.Context, address *net.UDPAddr, appKey string, key []byte) (io.WriteCloser, error) {
	connection, err := dtls.DialWithOptions("udp", address, dtls.WithPSK(func([]byte) ([]byte, error) { return key, nil }), dtls.WithPSKIdentityHint([]byte(appKey)), dtls.WithCipherSuites(dtls.TLS_PSK_WITH_AES_128_GCM_SHA256))
	if err != nil {
		return nil, err
	}
	handshake, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := connection.HandshakeContext(handshake); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return connection, nil
}

func (c *Client) StreamEntertainment(ctx context.Context, areaID string, frames []EntertainmentFrame, fps int) (sent int, resultErr error) {
	if fps < 1 || fps > 60 || len(frames) == 0 {
		return 0, fmt.Errorf("stream requires frames and a frame rate from 1 to 60")
	}
	if key, err := hex.DecodeString(c.bridge.ClientKey); err != nil || len(key) != 16 {
		return 0, fmt.Errorf("entertainment requires a valid client key; pair the bridge again with setup")
	}
	encoded := make([][]byte, len(frames))
	for i, frame := range frames {
		var err error
		encoded[i], err = EncodeEntertainmentFrame(areaID, byte(i%256), frame)
		if err != nil {
			return 0, err
		}
	}
	namespace, err := c.Resources.Of("entertainment_configuration", false)
	if err != nil {
		return 0, err
	}
	response, err := namespace.Write(ctx, "PUT", areaID, json.RawMessage(`{"action":"start"}`))
	if err != nil {
		return 0, err
	}
	if err := response.Err(); err != nil {
		return 0, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		response, err := namespace.Write(cleanup, "PUT", areaID, json.RawMessage(`{"action":"stop"}`))
		if err == nil {
			err = response.Err()
		}
		resultErr = errors.Join(resultErr, err)
	}()
	connection, err := dialEntertainment(ctx, c.bridge)
	if err != nil {
		return 0, err
	}
	defer func() { _ = connection.Close() }()
	stopClose := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stopClose()
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()
	for _, frame := range encoded {
		select {
		case <-ctx.Done():
			return sent, ctx.Err()
		case <-ticker.C:
		}
		if deadline, ok := connection.(interface{ SetWriteDeadline(time.Time) error }); ok {
			if err := deadline.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
				return sent, err
			}
		}
		if _, err := connection.Write(frame); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}
