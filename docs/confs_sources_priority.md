# VSS Configuration Priority

VSS can load configuration values from 3 sources:

1. Command line arguments
2. Environment variables
3. Configuration file

When the same setting is defined in more than one source, VSS uses the following priority order:

1. Command line arguments (highest priority)
2. Environment variables
3. Configuration file (lowest priority)

In practice, this means:

- A command line argument always overrides the same setting from environment variables and configuration files.
- An environment variable overrides the same setting from the configuration file.
- The configuration file is used as the base/default source.

## Example

If the same key is set in all 3 places:

- Config file: `PORT=8080`
- Environment variable: `PORT=9090`
- Command line argument: `--port=10000`

VSS will run with port `10000`.

If there is no command line argument, VSS will use `9090`.
If there is no command line argument and no environment variable, VSS will use `8080`.
