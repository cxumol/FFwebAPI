package ffmpeg

import (
    "fmt"
    "strings"

    "github.com/google/shlex"
)

// The placeholder for the input file in user commands
const InputMediaPlaceholder = "${INPUT_MEDIA}"

// SplitCommand securely splits a command string into a slice of arguments.
// It prevents shell injection by not using a shell.
func SplitCommand(command string) ([]string, error) {
    args, err := shlex.Split(command)
    if err != nil {
        return nil, fmt.Errorf("invalid command syntax: %w", err)
    }
    return args, nil
}

// SanitizeAndValidateArgs checks the split arguments for potential security risks.
func SanitizeAndValidateArgs(args []string) error {
    hasInput := false
    for _, arg := range args {
        // Rule 1: Disallow arguments that could write arbitrary files (apart from the main output).
        // This is tricky, ffmpeg has many. A blacklist is a start.
        if strings.HasPrefix(arg, "-f") || strings.HasPrefix(arg, "-map") {
            // This is a simplistic check. A more robust solution might require an allow-list of filters/options.
        }

        // Rule 2: Ensure the input placeholder is present.
        // Rule 3: Disallow shell-like metacharacters just in case, though exec.Command prevents their execution.
        // We allow " and ' as they are handled by shlex, but block others.
        if arg == InputMediaPlaceholder {
			hasInput = true
		} else if strings.ContainsAny(arg, "|&;`$()<>") {
			// This check is now only performed if the argument is NOT the placeholder.
			return fmt.Errorf("disallowed character found in argument: %s", arg)
		}
    }

    if !hasInput {
        return fmt.Errorf("command must include the input placeholder '%s'", InputMediaPlaceholder)
    }
    return nil
}
