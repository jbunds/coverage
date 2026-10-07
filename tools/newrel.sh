#!/usr/bin/env bash

latest_tag=$(git tag -l --sort=-v:refname | head -1)

semver='^v?[0-9]+\.[0-9]+\.[0-9]+$'

if [[ ! $latest_tag =~ $semver ]]; then
  echo "$latest_tag is not a valid semver tag"
	exit 1
fi

echo "latest tag: $latest_tag"

IFS=. read -r major minor patch <<< ${latest_tag#v}

new_patch=$((patch + 1))
new_tag=v${major}.${minor}.${new_patch}

read -r -n1 -p "proceed to release $new_tag ? [y/n] " response
echo

if [[ $response =~ ^[yY]$ ]]; then
  echo "proceeding with release..."
else
  echo "exiting"
	exit 1
fi

git tag $new_tag

git push origin $new_tag

git tag -fa v1 $new_tag -m "update v1 to point to $new_tag"

git push origin v1 --force

gh release create $new_tag --generate-notes
