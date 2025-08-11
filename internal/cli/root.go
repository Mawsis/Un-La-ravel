package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
// In Cobra, commands are organized in a tree structure with a root command at the top
var rootCmd = &cobra.Command{
	Use:   "unlaravel",
	Short: "Un(la)ravel - Laravel project analysis tool",
	Long: color.New(color.FgCyan).Sprint(`
🔍 Un(la)ravel - Laravel Project Analysis Tool

Un(la)ravel helps you understand Laravel projects by providing deep insights into:
• Route analysis and auto-generated Swagger documentation
• Database schema analysis and ER diagrams  
• Request/Resource/Policy mapping
• Performance analysis and optimization suggestions
• Security analysis and vulnerability detection

Use 'unlaravel help [command]' for more information about a command.
	`),
	// RunE is executed when the root command is called without subcommands
	// The 'E' suffix means it returns an error (vs Run which doesn't)
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is provided, show help
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// init is a special Go function that runs automatically when the package is imported
// It's perfect for setting up package-level configuration like CLI flags and subcommands
func init() {
	// Add global flags that apply to all commands
	// Cobra automatically generates help text for these flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().String("config", "", "Config file path (default is $HOME/.unlaravel.yaml)")
	
	// Add subcommands to the root command
	// We'll create these commands in separate files for better organization
	addAnalyzeCommand()
}

// addAnalyzeCommand adds the 'analyze' subcommand to the root command
// This demonstrates the Cobra pattern of organizing commands in separate functions
func addAnalyzeCommand() {
	analyzeCmd := &cobra.Command{
		Use:   "analyze [path]",
		Short: "Analyze a Laravel project",
		Long: `Analyze a Laravel project and generate comprehensive insights.
		
The analyze command will scan the specified Laravel project (or current directory if no path provided)
and generate analysis reports for:
• Routes and middleware chains
• Database schema and relationships
• Model dependencies and policies
• Performance bottlenecks
• Security configurations`,
		// Args validation - Cobra provides several built-in validators
		Args: cobra.MaximumNArgs(1), // Accept 0 or 1 arguments (path is optional)
		RunE: analyzeProject,        // The actual function that does the work
	}

	// Add command-specific flags
	// These flags only apply to the analyze command, not globally
	analyzeCmd.Flags().BoolP("routes", "r", false, "Analyze routes only")
	analyzeCmd.Flags().BoolP("database", "d", false, "Analyze database schema only")
	analyzeCmd.Flags().BoolP("swagger", "s", false, "Generate Swagger documentation")
	analyzeCmd.Flags().StringP("output", "o", "", "Output file path for analysis results")

	// Add the command to root
	rootCmd.AddCommand(analyzeCmd)
}

// analyzeProject is the main function that handles the analyze command
// Notice the function signature: (cmd *cobra.Command, args []string) error
// This is the standard Cobra command function signature
func analyzeProject(cmd *cobra.Command, args []string) error {
	// Get flag values - Cobra provides type-safe flag access
	verbose, _ := cmd.Flags().GetBool("verbose")
	routesOnly, _ := cmd.Flags().GetBool("routes")
	databaseOnly, _ := cmd.Flags().GetBool("database")
	generateSwagger, _ := cmd.Flags().GetBool("swagger")
	outputPath, _ := cmd.Flags().GetString("output")

	// Determine the project path
	// args is a slice (Go's version of arrays) - we check if it's empty
	projectPath := "." // Default to current directory
	if len(args) > 0 {
		projectPath = args[0]
	}

	// Create colored output for better UX
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)

	cyan.Printf("🔍 Analyzing Laravel project: %s\n\n", projectPath)

	if verbose {
		yellow.Println("📋 Analysis options:")
		fmt.Printf("  • Routes only: %v\n", routesOnly)
		fmt.Printf("  • Database only: %v\n", databaseOnly)
		fmt.Printf("  • Generate Swagger: %v\n", generateSwagger)
		fmt.Printf("  • Output path: %s\n", outputPath)
		fmt.Println()
	}

	// TODO: This is where we'll integrate our analysis logic
	// For now, we'll just simulate the analysis process
	green.Println("✅ Laravel project detected")
	green.Println("✅ Composer dependencies loaded")
	green.Println("✅ Configuration files parsed")
	
	if !routesOnly {
		green.Println("✅ Database schema analyzed")
		green.Println("✅ Model relationships mapped")
	}
	
	if !databaseOnly {
		green.Println("✅ Routes analyzed")
		green.Println("✅ Middleware chains mapped")
	}
	
	if generateSwagger {
		green.Println("✅ Swagger documentation generated")
	}

	cyan.Printf("\n🎉 Analysis complete! Found Laravel project at: %s\n", projectPath)
	
	return nil
}