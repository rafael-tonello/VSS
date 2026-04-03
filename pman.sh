#!/bin/bash
#this script contains project commands and helpers for development


#project commands {
    init_helpText="Initialize the project for development (install git hooks, etc.). Run it before starting daily development."
    init(){
        internalInit
        
        #open readme.md with code, gedit or xdg-open, in this order of preference
        if command -v code &> /dev/null; then
            code "README.md" &> /dev/null
        elif command -v gedit &> /dev/null; then
            gedit "README.md" &> /dev/null
        elif command -v xdg-open &> /dev/null; then
            xdg-open "README.md" &> /dev/null
        else
            echo ""
            echo "Important! Please open 'README.md' in your preferred text editor to view the project documentation."
            echo ""
        fi
    }

    finalize_helpText="Finalize the project after development. Run it after finishing daily development."
    finalize(){
        :;
    }

    new_version_helpText="Apply or create a new version. This function, besides be available as a project command, is called automatically after merging or commiting to main."
    new-version(){
        source $projectRootPath/pman/apply-or-create-new-version.sh "$@"
        return $?
    }


    build_helpArgs="[--debug]"
    build_helpText="Build the project. The option --debug generate a binary with debug symbols."
    build(){
        local debugMode=${1:-"--release"}

        copyAssets "$projectRootPath/sources/assets" "$projectRootPath/build"


        if [ "$debugMode" == "--release" ]; then
            echo building 'release version...'
            go build -o ./build/vss -ldflags="-s -w" ./main.go 2> /tmp/err.log
            if [ $? -ne 0 ]; then
                misc.PrintError "Build failed: $(cat /tmp/err.log)\n"
                return 1
            fi
        elif [ "$debugMode" == "--debug" ]; then
            echo building 'debug version...'
            go build -o ./build/vss -gcflags="all=-N -l" ./main.go 2> /tmp/err.log
            if [ $? -ne 0 ]; then
                misc.PrintError "Build failed: $(cat /tmp/err.log)\n"
                return 1
            fi
        else
            echo "Invalid option: $debugMode"
            echo "Use --debug for debug build or --release for release build"
            return 1
        fi
    }

    clean_helpText="Clean the project build files."
    clean(){
        rm -rf "$projectRootPath/build"
        misc.PrintGreen "Build files removed successfully\n"
    }

#}

#git hooks {
    onCommitMsg(){
        COMMIT_MSG_FILE="$1"
        COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")


        if grep -qE '^Merge ' "$COMMIT_MSG_FILE"; then
            return 0
        fi


        if ! echo "$COMMIT_MSG" | grep -qE '^(feat|fix|patch|chore|docs|refactor|test|perf|ci|build|revert|style|doc|BREAKING CHANGE):'; then
            echo "❌ wrong commit message format."
            echo "Use one of the following prefixes:"
            echo "  - feat: for new features"
            echo "  - fix: for bug fixes"
            echo "  - patch: for small bug fixes or improvements"
            echo "  - chore: for maintenance tasks"
            echo "  - docs: for documentation changes"
            echo "  - refactor: for code refactoring"
            echo "  - test: for adding or updating tests"
            echo "  - perf: for performance improvements"
            echo "  - ci: for continuous integration changes"
            echo "  - build: for build system changes"
            echo "  - revert: for reverting changes"
            echo "  - style: for code style changes (formatting, missing semi-colons, etc.)"
            echo "  - doc: for documentation only changes"
            echo "  - BREAKING CHANGE: for changes that break backward compatibility"
            echo "Example: 'feat: Add new user authentication feature'"
            
            return 1
        fi

    }

    beforeCommit(){
        :;
    }
    
    afterCommit(){
        checkMain(){
            #if commit to main, runs ./shu/tools/apply-or-create-new-version.sh
            if [ "$(git rev-parse --abbrev-ref HEAD)" = "main" ]; then
                #source ./shu/tools/apply-or-create-new-version.sh
                new-version
            fi
        }
        checkMain
    }

    afterMerge(){
        COMMIT_MSG_FILE="$1"
        COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")



        checkMain(){
            #if commit to main, runs ./shu/tools/apply-or-create-new-version.sh
            if [ "$(git rev-parse --abbrev-ref HEAD)" = "main" ]; then
                #source ./shu/tools/apply-or-create-new-version.sh
                new-version
            fi
        }
        checkMain
    }
#}

#others and internalfunctions {
    copyAssets(){
        local sourceDir="$1"
        local targetDir="$2"

        if [ -d "$sourceDir" ]; then
            mkdir -p "$targetDir"
        fi

        for file in "$sourceDir"/*; do
            #if is a folder copy it recursively
            if [ -d "$file" ]; then
                copyAssets "$file" "$targetDir/$(basename "$file")"
                continue
            fi

            #if file in the source dir is newer than the file in the target dir, copy it
            if [ ! -f "$targetDir/$(basename "$file")" ] || [ "$file" -nt "$targetDir/$(basename "$file")" ]; then
                 cp "$file" "$targetDir"
            else
                misc.PrintYellow "Asset file '$file' was changed in the destination folder, skipping copy...\n"
            fi
        done
    }

    setv(){
        local name="$1"
        local value="$2"

        local scriptFName="$(basename "$0")"
        local withNoExt="${scriptFName%.*}"
        mkdir -p $projectRootPath/.$withNoExt/vars/
        local varsFile="$projectRootPath/.$withNoExt/vars/$name"
        echo "$value" > "$varsFile"
        _error=""
    }

    getv(){
        local name="$1"

        local scriptFName="$(basename "$0")"
        local withNoExt="${scriptFName%.*}"
        local varsFile="$projectRootPath/.$withNoExt/vars/$name"
        if [ -f "$varsFile" ]; then
            _r=$(cat "$varsFile")
            _error=""
        else
            _r=""
            _error="variable '$name' not found"
        fi
    }

    #help text{
        help(){
            echo "Devhelper script for project at: $projectRootPath"
            echo ""
            echo "Usage: devhelper.sh <command> [args...]"
            echo ""
            echo "Available commands:"
            for cmd in $(compgen -A function); do
                local helpVar="${cmd}_helpText"
                #replace '-' by '_'
                helpVar="${helpVar//-/_}"
                #check if '$helpVar' contains '.' , if so, skip it (internal function)
                if [[ "$helpVar" == *.* ]]; then
                    continue
                fi

                local helpArgsVar="${cmd}_helpArgs"
                #replace '-' by '_'
                helpArgsVar="${helpArgsVar//-/_}"

                
                if [ ! -z "${!helpVar}" ]; then
                    printf "  %-20s %s\n\n" "$cmd ${!helpArgsVar}" "${!helpVar}"
                fi
            done
            echo ""
        }
    #}

    internalInit(){
        installGitHooks
        addHidenFolderToGitIgnore
    }

    addHidenFolderToGitIgnore(){
        local scriptFName="$(basename "$0")"
        local withNoExt="${scriptFName%.*}"
        local hideenFolder="$projectRootPath/.$withNoExt"

        if ! grep -q "$hideenFolder" .gitignore 2> /dev/null; then
            echo "$hideenFolder/" >> .gitignore
        fi
    }

    installGitHooks(){
        scriptFName="$(basename "$0")"
        #check if hooks are already installed
        misc.PrintYellow "Installing git hooks..."
        mkdir -p .git/hooks

        #commit-msg
        echo "#!/bin/bash" > .git/hooks/commit-msg
        echo "source \"$projectRootPath/$scriptFName\" onCommitMsg \"\$@\"" >> .git/hooks/commit-msg
        chmod +x .git/hooks/commit-msg

        #before-commit
        echo "#!/bin/bash" > .git/hooks/pre-commit
        echo "source \"$projectRootPath/$scriptFName\" beforeCommit \"\$@\"" >> .git/hooks/pre-commit
        chmod +x .git/hooks/pre-commit

        #after-commit
        echo "#!/bin/bash" > .git/hooks/post-commit
        echo "source \"$projectRootPath/$scriptFName\" afterCommit \"\$@\"" >> .git/hooks/post-commit
        chmod +x .git/hooks/post-commit

        #after-merge
        echo "#!/bin/bash" > .git/hooks/post-merge
        echo "source \"$projectRootPath/$scriptFName\" afterMerge \"\$@\"" >> .git/hooks/post-merge
        chmod +x .git/hooks/post-merge

        misc.PrintYellow " ok\n"
    }

    #source, using curl, https://raw.githubusercontent.com/rafael-tonello/SHU/refs/heads/main/src/shellscript-fw/common/misc.sh
    source <(curl -s https://raw.githubusercontent.com/rafael-tonello/SHU/refs/heads/main/src/shellscript-fw/common/misc.sh)
    devHelperPath="$(realpath "${BASH_SOURCE[0]}")"
    projectRootPath="$(dirname "$devHelperPath")"

    funcName="$1"
    if [ "$funcName" == "" ] || [ "$funcName" == "--help" ] || [ "$funcName" == "-h" ]; then
        help;
        exit 0
    fi

    #check if function exists
    if declare -f "$funcName" > /dev/null; then
        #call the function with all the remaining arguments
        shift
        "$funcName" "$@"
        #if file was sources, use return instead of exit
        if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
            return $?
        fi
        exit $?
    else
        misc.PrintError "Function '$funcName' not found in devhelper.sh"
        #if file was sources, use return instead of exit
        if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
            return $?
        fi
        exit 1
    fi
#}