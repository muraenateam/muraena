#!/bin/bash
# Thank you @evilsocket for this script ❤️
# nothing to see here, just a utility i use to create new releases ^_^

CURRENT_VERSION=$(cat core/banner.go | grep Version | cut -d '"' -f 2)
TO_UPDATE=(
    core/banner.go
)

echo -n "Current version is $CURRENT_VERSION, select new version: "
read NEW_VERSION
echo -e "Creating version $NEW_VERSION ...\n"

for file in "${TO_UPDATE[@]}"
do
    echo "Patching $file ..."
    gsed -i "s/$CURRENT_VERSION/$NEW_VERSION/g" $file
    git add $file
done

git commit -m "Releasing v$NEW_VERSION"
git push

echo
echo "Releasing v$NEW_VERSION initiated. CircleCI will do the magic now, hopefully!"
