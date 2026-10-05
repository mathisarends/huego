package hue

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/mathisarends/huego/internal/config"
	"github.com/pion/dtls/v3"
)

func TestEntertainmentWireFormat(t *testing.T) {
	id := "01234567-89ab-cdef-0123-456789abcdef"
	frame, err := EncodeEntertainmentFrame(id, 255, EntertainmentFrame{"2": {1, 0, 0.5}})
	if err != nil {
		t.Fatal(err)
	}
	expected := append([]byte("HueStream"), 2, 0, 255, 0, 0, 0, 0)
	expected = append(expected, id...)
	expected = append(expected, 2, 255, 255, 0, 0, 128, 0)
	if !bytes.Equal(frame, expected) {
		t.Fatalf("frame=%x want=%x", frame, expected)
	}
	for _, colors := range []EntertainmentFrame{{}, {"01": {1, 0, 0}}, {"256": {1, 0, 0}}, {"1": {1, 0}}, {"1": {1, -0.1, 0}}} {
		if _, err := EncodeEntertainmentFrame(id, 0, colors); err == nil {
			t.Fatal("invalid frame accepted", colors)
		}
	}
}

func TestEntertainmentDTLSHandshake(t *testing.T) {
	key := []byte("0123456789abcdef")
	listener, err := dtls.ListenWithOptions("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")}, dtls.WithPSK(func(identity []byte) ([]byte, error) {
		if string(identity) != "app-key" {
			return nil, fmt.Errorf("unexpected identity")
		}
		return key, nil
	}), dtls.WithPSKIdentityHint([]byte("bridge")), dtls.WithCipherSuites(dtls.TLS_PSK_WITH_AES_128_GCM_SHA256))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 128)
		n, err := conn.Read(buf)
		if err == nil && string(buf[:n]) != "test-frame" {
			err = fmt.Errorf("frame changed in transit")
		}
		result <- err
	}()
	conn, err := connectEntertainment(ctx, listener.Addr().(*net.UDPAddr), "app-key", key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("test-frame")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

type frameSink struct {
	bytes.Buffer
	closed   bool
	writeErr error
}

func (s *frameSink) Close() error { s.closed = true; return nil }
func (s *frameSink) Write(data []byte) (int, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	return s.Buffer.Write(data)
}

func TestEntertainmentAlwaysReleasesStartedArea(t *testing.T) {
	for _, fail := range []string{"", "dial", "write"} {
		t.Run(fail, func(t *testing.T) {
			var actions []string
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				actions = append(actions, string(body))
				_, _ = io.WriteString(w, `{"errors":[],"data":[]}`)
			})
			client.bridge.ClientKey = "00112233445566778899aabbccddeeff"
			original := dialEntertainment
			t.Cleanup(func() { dialEntertainment = original })
			sink := &frameSink{}
			if fail == "write" {
				sink.writeErr = errors.New("write failed")
			}
			dialEntertainment = func(context.Context, config.Bridge) (io.WriteCloser, error) {
				if fail == "dial" {
					return nil, errors.New("dial failed")
				}
				return sink, nil
			}
			sent, err := client.StreamEntertainment(t.Context(), "01234567-89ab-cdef-0123-456789abcdef", []EntertainmentFrame{{"0": {1, 0, 0}}}, 60)
			if len(actions) != 2 || actions[0] != `{"action":"start"}` || actions[1] != `{"action":"stop"}` {
				t.Fatal(actions)
			}
			if fail == "" && (err != nil || sent != 1 || !sink.closed) {
				t.Fatal(sent, err, sink.closed)
			}
			if fail != "" && err == nil {
				t.Fatal("failure was hidden")
			}
		})
	}
}
