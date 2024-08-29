package cmd

// var completionCmd = &cobra.Command{
// 	Use:   "completion [bash|zsh|fish|powershell]",
// 	Short: "Generate completion script",
// 	Long: `To load completions:

// Bash:
//   $ source <(andaime completion bash)

// Zsh:
//   # If shell completion is not already enabled in your environment,
//   # you will need to enable it.  You can execute the following once:

//   $ echo "autoload -U compinit; compinit" >> ~/.zshrc

//   # To load completions for each session, execute once:
//   $ andaime completion zsh > "${fpath[1]}/_andaime"

//   # You will need to start a new shell for this setup to take effect.

// fish:
//   $ andaime completion fish | source

//   # To load completions for each session, execute once:
//   $ andaime completion fish > ~/.config/fish/completions/andaime.fish

// PowerShell:
//   PS> andaime completion powershell | Out-String | Invoke-Expression

//   # To load completions for every new session, run:
//   PS> andaime completion powershell > andaime.ps1
//   # and source this file from your PowerShell profile.
// `,
// 	DisableFlagsInUseLine: true,
// 	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
// 	Args:                  cobra.ExactValidArgs(1),
// 	Run: func(cmd *cobra.Command, args []string) {
// 		switch args[0] {
// 		case "bash":
// 			cmd.Root().GenBashCompletion(os.Stdout)
// 		case "zsh":
// 			cmd.Root().GenZshCompletion(os.Stdout)
// 		case "fish":
// 			cmd.Root().GenFishCompletion(os.Stdout, true)
// 		case "powershell":
// 			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
// 		}
// 	},
// }

// func getCompletionCmd() *cobra.Command {
// 	return completionCmd
// }
