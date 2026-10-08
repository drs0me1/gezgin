package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/filebrowser/filebrowser/v2/trash"
)

func init() {
	usersCmd.AddCommand(usersRmCmd)
}

var usersRmCmd = &cobra.Command{
	Use:   "rm <id|username>",
	Short: "Delete a user by username or id",
	Long:  `Delete a user by username or id`,
	Args:  cobra.ExactArgs(1),
	RunE: withViperAndStore(func(_ *cobra.Command, args []string, v *viper.Viper, st *store) error {
		username, id := parseUsernameOrID(args[0])
		var err error

		if username != "" {
			id, err = st.DeleteUser(username)
		} else {
			id, err = st.DeleteUser(id)
		}

		if err != nil {
			return err
		}
		fmt.Println("user deleted successfully")

		// The user's trash goes with them (Gezgin).
		server, err := getServerSettings(v, st.Storage)
		if err == nil {
			err = trash.For(server.Root, id).Empty()
		}
		if err != nil {
			fmt.Printf("the user's trash could not be emptied: %v\n", err)
		}
		return nil
	}, storeOptions{}),
}
