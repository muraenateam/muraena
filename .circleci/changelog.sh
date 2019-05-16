#!/bin/bash
# Thank you @evilsocket for this script ❤️

NEW=()
FIXES=()
MISC=()

echo "@ Fetching remote tags ..."

git fetch --tags > /dev/null

CURTAG=$(git describe --tags --abbrev=0)
OUTPUT=$(git log $CURTAG..HEAD --oneline)
IFS=$'\n' LINES=(${OUTPUT})

for LINE in "${LINES[@]}"; do
    LINE=$(echo "$LINE" | sed -E "s/^[[:xdigit:]]+\s+//")
    if [[ $LINE = *"new:"* ]]; then
        LINE=$(echo "$LINE" | sed -E "s/^new: //")
        NEW+=("$LINE")
    elif [[ $LINE = *"fix:"* ]]; then
        LINE=$(echo "$LINE" | sed -E "s/^fix: //")
        FIXES+=("$LINE")
    elif [[ $LINE != *"Merge "* ]] && [[ $LINE != "Releasing"* ]]; then
        echo "MISC LINE =$LINE"
        LINE=$(echo "$LINE" | sed -E "s/^[a-z]+: //")
        MISC+=("$LINE")
    fi
done

echo > CHANGELOG.md
echo "Changelog" >> CHANGELOG.md
echo "===" >> CHANGELOG.md
if [[ -n "$NEW" ]]; then
    echo -e "\n**New Features**\n" >> CHANGELOG.md
    for l in "${NEW[@]}"
    do
        echo "* $l" >> CHANGELOG.md
    done
fi

if [[ -n "$FIXES" ]]; then
    echo -e "\n**Fixes**\n" >> CHANGELOG.md
    for l in "${FIXES[@]}"
    do
        echo "* $l" >> CHANGELOG.md
    done
fi

if [[ -n "$MISC" ]]; then
    echo -e "\n**Misc**\n" >> CHANGELOG.md
    for l in "${MISC[@]}"
    do
        echo "* $l" >> CHANGELOG.md
    done
fi

