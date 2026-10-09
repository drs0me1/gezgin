package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

func init() {
	configCmd.AddCommand(configSetCmd)
	addConfigFlags(configSetCmd.Flags())
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Updates the configuration",
	Long: `Updates the configuration. Set the flags for the options
you want to change. Other options will remain unchanged.`,
	Args: cobra.NoArgs,
	RunE: withStore(func(cmd *cobra.Command, _ []string, st *store) error {
		flags := cmd.Flags()

		// Read existing config
		set, err := st.Settings.Get()
		if err != nil {
			return err
		}

		ser, err := st.Settings.GetServer()
		if err != nil {
			return err
		}

		// A method Gezgin dropped (noauth, hook, proxy) can only be replaced, with --auth.method.
		auther, err := st.Auth.Get(set.AuthMethod)
		if err != nil && (!errors.Is(err, fberrors.ErrInvalidAuthMethod) || !flags.Changed("auth.method")) {
			return err
		}

		// Get updated config
		auther, err = getSettings(flags, set, ser, auther, false)
		if err != nil {
			return err
		}

		// Save updated config
		err = st.Auth.Save(auther)
		if err != nil {
			return err
		}

		err = st.Settings.Save(set)
		if err != nil {
			return err
		}

		err = st.Settings.SaveServer(ser)
		if err != nil {
			return err
		}

		return printSettings(ser, set, auther)
	}, storeOptions{}),
}
