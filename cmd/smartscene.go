package cmd

import (
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type smartSceneView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Group  string `json:"group,omitempty"`
	Target string `json:"target"`
	State  string `json:"state"`
}

func newSmartSceneView(s hue.SmartScene) smartSceneView {
	return smartSceneView{ID: s.ID, Name: s.GetName(), Group: s.GroupName, Target: s.QualifiedName(), State: string(s.State)}
}

var smartSceneListing = listing[smartSceneView]{
	headers: []string{"NAME", "GROUP", "STATE", "ID"},
	row: func(v smartSceneView) []string {
		return []string{v.Name, orDash(v.Group), orDash(v.State), v.ID}
	},
	name: func(v smartSceneView) string { return v.Target },
}

func (v smartSceneView) fields() []field {
	return []field{
		{"Name", v.Name},
		{"ID", v.ID},
		{"Group", v.Group},
		{"State", v.State},
	}
}

var smartSceneCmd = &cobra.Command{
	Use:     "smart-scene",
	Aliases: []string{"smart-scenes", "smartscene"},
	Short:   "Inspect, start, and stop smart scenes that follow the time of day",
}

var smartSceneListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List smart scenes with the room or zone they belong to",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		scenes, err := client.SmartScenes.ListWithGroupNames(cmd.Context())
		if err != nil {
			return err
		}
		if group, _ := cmd.Flags().GetString("group"); group != "" {
			if scenes, err = inGroup(scenes, group, "smart scenes", func(s hue.SmartScene) string { return s.GroupName }); err != nil {
				return err
			}
		}
		return writeList(cmd, mapViews(scenes, newSmartSceneView), smartSceneListing)
	},
}

var smartSceneGetCmd = &cobra.Command{
	Use:   "get <target>",
	Short: "Show one smart scene",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := client.SmartScenes.ResolveTarget(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		scenes, err := client.SmartScenes.ListWithGroupNames(cmd.Context())
		if err != nil {
			return err
		}
		scene, err := findByID(scenes, id, "smart scene")
		if err != nil {
			return err
		}
		view := newSmartSceneView(scene)
		return writeDetails(cmd, view, view.fields())
	},
}

func newSmartSceneSwitchCommand(use, short string, active bool) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <target>",
		Short: short,
		Long:  short + ".\ntarget is a UUID, a name, or \"<room or zone>/<smart scene>\".",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			id, err := client.SmartScenes.ResolveTarget(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			resp, err := client.SmartScenes.SetActive(cmd.Context(), id, active)
			if err != nil {
				return err
			}
			return writeResponse(cmd, args[0], resp)
		},
	}
}

func init() {
	addNamesFlag(smartSceneListCmd)
	smartSceneListCmd.Flags().String("group", "", "only smart scenes of this room or zone")
	smartSceneCmd.AddCommand(
		smartSceneListCmd,
		smartSceneGetCmd,
		newSmartSceneSwitchCommand("activate", "Start a smart scene", true),
		newSmartSceneSwitchCommand("deactivate", "Stop a smart scene and leave its lights as they are", false),
	)
	rootCmd.AddCommand(smartSceneCmd)
}
