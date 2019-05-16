#!/bin/bash
# nothing to see here, just a utility I use to create releases via CircleCI ^_^

VERSION=$(cat core/banner.go | grep Version | cut -d '"' -f 2)

cat << EOF >> RELEASE.md
Hello,
 a new version is out: **v${VERSION} 🎉**

<center>
<img src="https://avatars1.githubusercontent.com/u/50457173?s=150&u=b2fe8f4d050dfea26f0391aa210e0a462d8d478f&v=4"/>
</center>

EOF

bash $(dirname "$0")/changelog.sh
cat CHANGELOG.md >> RELEASE.md

echo -e "## SHA256\n" >> RELEASE.md
echo '```' >> RELEASE.md
for file in "$@"; do
    HASH=$(sha256sum $file | cut -d " " -f 1)
    NAME=$(basename $file)
    echo "$HASH $NAME" >> RELEASE.md
done
echo '```' >> RELEASE.md