#!/bin/bash
#this script contains project commands and helpers for development

#project dependencies {
    # * use addDep to append deps
    # * one dependency per line
    # * do not remove markers
    # * format: "git_url|branch_or_commit|folder_in_git_repo|destination_folder"
    dependencies=(
        #--begin-dependencies
        
        #--end-dependencies
    )
#}

#project commands {
    init_helpArgs="[--force | -f]"
    init_helpText="Initialize the project for development (install git hooks, etc.). Run it before starting daily development. --force option ignores previous initialization and forces it to run again."
    init(){
        getv "initialized"; local initialized="$_r"
        if [ "$initialized" == "true" ]; then
            local force="false"
            #check if any arg is --force or -f
            if [[ " $@ " == *" --force "* ]] || [[ " $@ " == *" -f "* ]]; then
                force="true"
            fi

            if [ "$force" == "true" ]; then
                misc.PrintYellow "Forcing initialization of an already initialized project...\n"
            else
                return 0
            fi
        fi
        internalInit
        setv "initialized" "true"
        
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

    finalize_helpText="Finalizes the project after development. Run it after finishing daily development."
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
            init        
            echo building 'release version...'
            go build -o ./build/vss -ldflags="-s -w" ./main.go 2> /tmp/err.log
            if [ $? -ne 0 ]; then
                misc.PrintError "Build failed: $(cat /tmp/err.log)\n"
                return 1
            fi
        elif [ "$debugMode" == "--debug" ]; then
            init
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

    clean_helpArgs="[--pman]"
    clean_helpText="Clean the project build files. . Use --pman to remove .pman.sh hidden folder (you should run 'init' command after that to reinitialize the project)."
    clean(){
        rm -rf "$projectRootPath/build"
        #check if any argument is '--pman'
        if [ "$@" == *"--pman"* ]; then
            local scriptFName="$(basename "$0")"
            local withNoExt="${scriptFName%.*}"
            rm -rf "$projectRootPath/.$withNoExt"
        fi

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


        if ! echo "$COMMIT_MSG" | grep -qE '^(feat|fix|patch|chore|docs|refactor|test|perf|ci|build|revert|style|doc|BREAKING CHANGE)( ?\([^)]+\))?:'; then
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

#dependency events {
    onNewDepAdded(){ local gitUrl="$1"; local destinationFolder="$2"
        if [ "$destinationFolder" == "" ]; then
            destinationFolder="./.pman/packages/$folderName"
        fi

        #add dependencies to platformio.ini
        folderName=$(basename "$gitUrl" .git)
        if ! grep -q "build_flags" platformio.ini 2> /dev/null; then
            echo "build_flags = -I .pman/packages/$folderName/src" >> platformio.ini
        else
            sed -i "/build_flags/ s|$| -I .pman/packages/$folderName/src|" platformio.ini
        fi
    }
#}

#others and internalfunctions {
    projectRootPath=$(realpath "./")

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
        mkdir -p "$projectRootPath/.$withNoExt/vars/"
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
            local identSize=${1:-"20"}
            local terminalWidth=$(tput cols)
            if [ $terminalWidth -gt 120 ]; then
                #try via stty size, if fails, set to 120
                terminalWidth=$(stty size 2> /dev/null | awk '{print $2}')
                if [ -z "$terminalWidth" ]; then
                    terminalWidth=100
                fi
            fi

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

                if [ -z "${!helpVar}" ]; then
                    continue;
                fi

                local helpText="${!helpVar}"

                local boldCmd="\033[1m$cmd\033[0m"
                local italicHelpArgs="\033[3m${!helpArgsVar}\033[0m"

                local header="$boldCmd $italicHelpArgs"
                local headerNoFormat="$cmd ${!helpArgsVar}"

                local totalHeaderSize=$(( ${#headerNoFormat} + 1 ))

                local identText="$(printf "%*s" "$((identSize + 3))" " ")"

                if [ "$totalHeaderSize" -lt $identSize ]; then
                    helpText="$(printf "  %-${identSize}s" "$header ") $helpText"
                else
                    printf "  $header\n"
                fi

                #spit by '\n' string (not the char) to get idividual lines
                
                local linesArr=()
                while true; do
                    if [[ "$helpText" == *"\n"* ]]; then
                        linesArr+=("${helpText%%\\n*}")
                        helpText="${helpText#*\\n}"
                    else
                        linesArr+=("$helpText")
                        break
                    fi
                done


                for line in "${linesArr[@]}"; do
                    #check if line is not the header
                    if [[ "$line" != *"$header"* ]]; then
                        line="$identText$line"
                    fi

                    while true; do
                        #break only on space or end of line, to avoid cutting words
                        local cutPosition=$terminalWidth
                        while true; do
                            charAtCutPos=$(echo "$line" | cut -c$cutPosition)
                            if [ "$charAtCutPos" == " " ] || [ "$charAtCutPos" == "" ]; then
                                break;
                            fi
                            cutPosition=$((cutPosition - 1))
                        done

                        toPrint=$(echo "$line" | cut -c1-$cutPosition)
                        line="${line:$cutPosition}"
                        #trim start
                        line="${line#"${line%%[![:space:]]*}"}"

                        printf "$toPrint\n"

                        if [ -z "$line" ]; then
                            break
                        fi

                        line="$identText $line"
                    done
                done
                    
                #if [ ! -z "${!helpVar}" ]; then
                #    printf "  %-20s %s\n\n" "$cmd ${!helpArgsVar}" "${!helpVar}"
                #fi

                echo ""
            done
            echo ""
        }
    #}

    internalInit(){
        installGitHooks
        addHidenFolderToGitIgnore

        mkdir -p "$projectRootPath/.pman"
        echo "# About this folder (.pman)" > "$projectRootPath/.pman/README.md"
        echo "This folder is used by the 'pman' command to store temporary files, variables and other data that should not be committed to git." >> "$projectRootPath/.pman/README.md"
        
        initDepSystem
    }

    addHidenFolderToGitIgnore(){
        local scriptFName="$(basename "$0")"
        local withNoExt="${scriptFName%.*}"
        #local hideenFolder="$projectRootPath/.$withNoExt"
        local hideenFolder=".$withNoExt"

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

    # dependency management system {
        thisScriptPath="$(realpath "${BASH_SOURCE[0]}")"

        addDep_helpText="Add a dependency to the project.The dependency should be a git repository. \nThe option branch_or_commit is optional, if not provided, the main branch will be used. destination_folder can be used if positioning the dependency in a specific folder is desired, if not provided, the dependency will be added to .pman/packages folder. \nThe option repo-subfolder is used when the desired dependency is in a subfolder of the git repository"
        addDep_helpArgs="<git_url> [--hash <branch_or_commit>] [--repo-subfolder <folder_in_git_repo>] [--destination <destination_folder>]"
        
        addDep(){ local gitUrl="$1"
            misc.GetArgByName "--hash" "" "$@"; local branchCommitOrTag="$_r"
            misc.GetArgByName "--repo-subfolder" "" "$@"; local gitRepoFolder="$_r"
            misc.GetArgByName "--destination" "" "$@"; local _destinationFolder="$_r"

            #save dependencie
            _registerDep "$gitUrl" "$branchCommitOrTag" "$gitRepoFolder" "$_destinationFolder"

            #if destination folder is not empty. add it to the .gitignore file
            if [ "$_destinationFolder" == "" ]; then
                _destinationFolder="./.pman/packages/$folderName"
            elif ! grep -q "$_destinationFolder" .gitignore 2> /dev/null; then
                echo "$_destinationFolder/" >> .gitignore
            fi
            
            _restoreDep "$gitUrl" "$branchCommitOrTag" "$gitRepoFolder" "$_destinationFolder"

            onNewDepAdded "$gitUrl" "$_destinationFolder"
            
        }

        _registerDep() {
            local dep="$1|$2|$3|$4"

            awk -v dep="$dep" '
            !inserted && /#--end-dependencies/ {
                print "            \"" dep "\""
                inserted=1
            }
            { print }
            ' "$0" > "$0.tmp" && mv "$0.tmp" "$0"

            chmod +x "$0"
        }

        _restoreDep(){ local gitUrl="$1"; local branchCommitOrTag="$2"; local gitRepoFolder="$3"; local destinationFolder="$4"
            folderName=$(basename "$gitUrl" .git)
            if [ -z "$branchCommitOrTag" ]; then
                branchCommitOrTag="main"
            fi

            local copyOrigin="/tmp/$folderName"
            if [ -n "$gitRepoFolder" ]; then
                copyOrigin+="/$gitRepoFolder"
                destinationFolder+="/$gitRepoFolder"
            fi

            #clone to /tmp
            git clone "$gitUrl" "/tmp/$folderName" --depth 1 --branch "$branchCommitOrTag" 2> /tmp/err.log 1> /dev/null
            if [ $? -ne 0 ]; then
                misc.PrintError "Failed to clone dependency '$gitUrl': $(cat /tmp/err.log)\n"
                return 1
            fi

            #copy files to .pman/packages
            rm -rf /tmp/"$folderName"/.git
            rm -rf "$destinationFolder"

            mkdir -p "$destinationFolder"

            cp -r "$copyOrigin"/* "$destinationFolder"/

            rm -rf /tmp/"$folderName"

            _r="$destinationFolder"
        }

        _restoreDeps(){ local pmanScriptName="${1:-$thisScriptPath}"
            #looks for the 'dependencies=(' line    
            local depsStartLine=$(grep -nm1 "dependencies=(" "$pmanScriptName" | cut -d: -f1)
            if [ -z "$depsStartLine" ]; then
                return
            fi

            

            #read lines until the ')', and save them in an array
            local dependencies=()
            local lineNum=$((depsStartLine + 1))
            while true; do
                local line=$(sed -n "${lineNum}p" "$pmanScriptName")
                #trim line start
                line="${line#"${line%%[![:space:]]*}"}"
                
                #check if line contains ')'
                if [[ "$line" == *")"* ]]; then
                    break
                fi

                #ignore if line starts with '#'
                if [[ "$line" == \#* ]]; then
                    lineNum=$((lineNum + 1))
                    continue
                fi

                #ignore empty lines
                if [[ -z "$line" ]]; then
                    lineNum=$((lineNum + 1))
                    continue
                fi

                #remove quotes form begin and end of line
                line="${line%\"}"
                line="${line#\"}"


                dependencies+=("$line")
                lineNum=$((lineNum + 1))
            done

            #check if project have any dependency
            if [ ${#dependencies[@]} -eq 0 ]; then
                misc.PrintGray "No dependencies found for this project. Run $0 --help to see how to add dependencies and other available commands.\n"
                return
            fi

            misc.PrintYellow "Restoring dependencies for ...\n"
            for dep in "${dependencies[@]}"; do
                local gitUrl=$(echo "$dep" | cut -d'|' -f1)
                local branchCommitOrTag=$(echo "$dep" | cut -d'|' -f2)
                local gitRepoFolder=$(echo "$dep" | cut -d'|' -f3)
                local destinationFolder=$(echo "$dep" | cut -d'|' -f4)

                #if destination folder is not empty. add it to the .gitignore file
                if [ "$destinationFolder" == "" ]; then
                    destinationFolder="./.pman/packages/$folderName"
                fi

                misc.PrintGray "   Dependency '$gitUrl' -> '$destinationFolder'..."

                _restoreDep "$gitUrl" "$branchCommitOrTag" "$gitRepoFolder" "$destinationFolder"
                if [ "$_error" != "" ]; then
                    misc.PrintError "Failed to restore dependency '$gitUrl': $_error\n"
                    continue
                fi
                misc.PrintGray "ok\n"

                #check if destination folder as a pman.sh file
                if [ -f "$destinationFolder/pman.sh" ]; then
                    _restoreDeps "$destinationFolder/pman.sh"
                fi
            done

            misc.PrintYellow "ok\n"
        }

        initDepSystem(){
            _error=""
            _restoreDeps
        }
    # }

    #source, using curl, https://raw.githubusercontent.com/rafael-tonello/SHU/refs/heads/main/src/shellscript-fw/common/misc.sh
    source <(curl -s https://raw.githubusercontent.com/rafael-tonello/SHU/refs/heads/main/common/misc.sh)
    devHelperPath="$(realpath "${BASH_SOURCE[0]}")"
    projectRootPath="$(dirname "$devHelperPath")"

    funcName="$1"
    if [ "$funcName" == "" ] || [ "$funcName" == "--help" ] || [ "$funcName" == "-h" ]; then
        help;
        exit 0
    fi

    #replace "-" with "_" in funcName
    funcName="${funcName//-/_}"

    #check if function exists
    if declare -f "$funcName" > /dev/null; then
        #call the function with all the remaining arguments
        shift
        "$funcName" "$@"; funcExitCode=$?
        #if file was sourced, use return instead of exit
        if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
            return $funcExitCode
        fi
        exit $funcExitCode
    else
        misc.PrintError "Function '$funcName' not found in devhelper.sh"
        #if file was sourced, use return instead of exit
        if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
            return 1
        fi
        exit 1
    fi
#}
