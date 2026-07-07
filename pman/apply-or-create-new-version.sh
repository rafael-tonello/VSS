#!/bin/bash

#this script does the following:
#1. Check if there are pending git changes, if there are, the scripts will fail
#3. Execute git checkout develop
#4. change the version number in the main.go file (looks for the line starting with 'string INFO_VERSION' and change it to 'string INFO_VERSION = "<version>"')
#5. execute git add ./main.go
#6. execute git commit -m "Changes version number to <version>"
#7. execute git push origin develop
#8. execute git checkout main
#9. execute git merge develop
#10. execute git push origin main
#11. execute git tag <version>
#12. execute git push origin <version>

# Força o uso do terminal físico do usuário
exec < /dev/tty

tryCreateVersion(){
    local lastTag=""
    local lastVersion=""
    local version="$1"
    local addInfo="$2"
    local major=0
    local minor=0
    local patch=0
    #get last git tag (look for the last tag in the remote repository)
    lastTag=$(git describe --tags --abbrev=0 origin 2>/dev/null)
    
    #if no tag found, may be the connection is down, so try to get the last tag from the local repository
    if [ -z "$lastTag" ]; then
        lastTag=$(git describe --tags --abbrev=0 2>/dev/null)
    fi

    lastVersion="$lastTag"
    #check if contains '+', if true, separate version from aditional info
    if [[ "$lastVersion" == *"+"* ]]; then
        addInfo="${lastVersion#*+}"  #everything after the first '+' character
        lastVersion="${lastVersion%%+*}"  #remove everything after the first '+' character
    fi

    #with the version, removes the v (if present) fro the beginning of the string
    lastVersion="${lastVersion#v}"  #remove 'v' from the beginning of the string
    #check if lastTag is empty
    if [ -z "$lastVersion" ]; then
        echo "No previous version found, using 0.0.0 as base version"
        lastVersion="0.0.0"
    fi

    #separate major, minor and patch version numbers from lastVersion
    major=$(echo "$lastVersion" | cut -d '.' -f 1)
    minor=$(echo "$lastVersion" | cut -d '.' -f 2)
    patch=$(echo "$lastVersion" | cut -d '.' -f 3)

    INCREMENT_MAJOR=false
    INCREMENT_MINOR=false
    INCREMENT_PATCH=false

    #check if ther eis any tag in the project
    if [ -z "$lastTag" ]; then
        _error="No tags found in the repository, cannot create a new version based on commits. Please specify a version number manually."
        _r=""
        return 1
    fi

    #scrolls commits from lastVersion to the last one
    local commits=$(git log --pretty=format:"%H" "$lastTag"..HEAD)
    for commit in $commits; do
        local message=$(git log -1 --pretty=%B $commit)

        if echo "$message" | grep -q "^BREAKING CHANGE"; then
            INCREMENT_MAJOR=true
        #check if thers is a '!' before the ':' in the commit message
        elif echo "$message" | grep -qE "^[a-zA-Z]+!:"; then
            INCREMENT_MAJOR=true
        elif echo "$message" | grep -q "^feat"; then
            INCREMENT_MINOR=true
        elif echo "$message" | grep -qE "^(fix|patch)"; then
            INCREMENT_PATCH=true
        fi
    done
    #increment version numbers
    local razon=""
    if [ "$INCREMENT_MAJOR" = true ]; then
        razon="Commits with BREAKING CHANGE"
        major=$((major + 1))
        minor=0
        patch=0
    elif [ "$INCREMENT_MINOR" = true ]; then
        razon="Commit(s) starting with 'feat'"
        ehco "inc minor"
        minor=$((minor + 1))
        patch=0
    elif [ "$INCREMENT_PATCH" = true ]; then
        razon="Commit(s) starting with 'fix'"
        echo "inc patch"
        patch=$((patch + 1))
    else
        _error="No commits found that require a version increment (no commits with 'feat', 'fix' or 'BREAKING CHANGE')"
        _r=""
        return 1
        
    fi

    #create new version string
    local newVersion="$major.$minor.$patch"
    if [ -n "$addInfo" ]; then
        newVersion="v$newVersion+$addInfo"
    fi

    echo "You didn't specify a version number, so a new one was created based on the last tag:"
    echo "   Last tag: $lastTag"
    echo "   New tag (with version): $newVersion"
    if [ -n "$razon" ]; then
        echo "   Reason for incrementing: $razon"
    fi

    echo "   Commits:"
    for commit in $commits; do
        local message=$(git log -1 --pretty=%B $commit)
        echo "      - $message"
    done
    echo "Do you want to use this version? (y/n)"

    read -r answer1
    if [[ "$answer1" =~ ^[Yy]$ ]]; then
        _error=""
        _r="$newVersion"
        return 0
    else
        _error="User declined to use the automatically generated version"
        _r=""
        return 1
    fi

    
}

main(){

    dryrun=false
    #check if argument --dry-run, --dryrun or -d was passed (in any position)
    for arg in "$@"; do
        if [[ "$arg" == "--dry-run" || "$arg" == "--dryrun" || "$arg" == "-d" ]]; then
            dryrun=true
            #remove the argument from the list of arguments
            set -- "${@/$arg/}"

            echo "remains arguments are: $@"
            break
        fi
    done

    version="$1"
    #check if version was passed as argument
    if [ -z "$version" ]; then
        echo "No version number supplied"
        tryCreateVersion
        if [ -n "$_error" ]; then
            misc.PrintError "Error: $_error"
            exit 1
        fi
        version="$_r"
    fi

    #check if there are pending changes
    if [ -n "$(git status --porcelain)" ]; then
        misc.PrintYellow "There are pending changes, please commit or stash them before running this script"
        git status
        exit 1
    fi

    #ask for confirmation
    echo -n "Are you sure you want to apply version $version? (y/n) " > /dev/tty
    read -r answer < /dev/tty
    if [[ ! "$answer" =~ ^[Yy]$ ]]; then
        echo -e "\nOperation cancelled."
        exit 0
    fi

    #checkout develop
    echo "Checking out develop branch..."
    git checkout develop

    #change version number
    echo "Changing version number to $version in ./main.go..."

    if [ $dryrun == false ]; then
        sed -i "s/const INFO_VERSION = \".*\"/const INFO_VERSION = \"$version\"/g" ./main.go
    fi

    #add commits since last tag to the CHANGELOG.md file
    lastTag=$(git describe --tags --abbrev=0 origin 2>/dev/null)
    if [ -z "$lastTag" ]; then
        lastTag=$(git describe --tags --abbrev=0 2>/dev/null)
    fi
    if [ -z "$lastTag" ]; then
        lastTag=$(git rev-list --max-parents=0 HEAD)
    fi

    if [ $dryrun == false ]; then
        echo "# Version $version changes" >> CHANGELOG.md
    else
        echo "dry run: mimicking 'echo \"# Version $version changes\" >> CHANGELOG.md'"
    fi

    for commit in $(git log --pretty=format:"%H" "$lastTag"..HEAD); do
        local message=$(git log -1 --pretty=%B $commit)

        #ignore merge commits
        if echo "$message" | grep -q "^Merge "; then
            continue

        #ignore 'new empty CHANGELOG.md' commits
        elif echo "$message" | grep -q "creates new empty CHANGELOG.md"; then
            continue
        fi

        echo "- $message" >> CHANGELOG.md
    done

    if [ $dryrun == false ]; then
        echo "" >> CHANGELOG.md
    else
        echo "dry run: mimicking 'add a new line to CHANGELOG.md'"
    fi

    #add changes
    echo "Adding changes to git..."

    if [ $dryrun == false ]; then
        git add ./main.go
        git add CHANGELOG.md
    else
        echo "dry run: mimicking 'git add ./main.go' and 'git add CHANGELOG.md'"
    fi

    #commit changes
    if [ $dryrun == false ]; then
        git commit -m "chore: changes version number to $version and updates CHANGELOG.md"
    else
        echo "dry run: mimicking 'git commit -m \"chore: changes version number to $version and updates CHANGELOG.md\"'"
    fi

    #push changes
    if [ $dryrun == false ]; then
        git push origin develop
    else
        echo "dry run: mimicking 'git push origin develop'"
    fi


    #checkout main
    echo "Checking out main branch..."
    git checkout main

    #merge develop
    echo "Merging develop branch into main..."
    if [ $dryrun == false ]; then
        git merge develop
    else
        echo "dry run: mimicking 'git merge develop'"
    fi

    #push changes
    echo "Pushing changes to origin main..."
    if [ $dryrun == false ]; then
        git push origin main
    else
        echo "dry run: mimicking 'git push origin main'"
    fi

    #create tag
    echo "Creating tag $version..."
    if [ $dryrun == false ]; then
        git tag $version
    else
        echo "dry run: mimicking 'git tag $version'"
    fi

    #push tag
    echo "Pushing tag $version to origin..."
    if [ $dryrun == false ]; then
        git push origin $version
    else
        echo "dry run: mimicking 'git push origin $version'"
    fi

    #checkout develop
    git checkout develop

    if [ $dryrun == false ]; then
        echo "" > CHANGELOG.md
        git add CHANGELOG.md
        git commit -m "chore: creates new empty CHANGELOG.md for next version"
        git push origin develop
    else
        echo "dry run: mimicking 'create new empty CHANGELOG.md, git add, git commit and git push origin develop'"
    fi

    echo "Version $version applied successfully and new CHANGELOG.md created."
    exit 0
}

main "$@"
