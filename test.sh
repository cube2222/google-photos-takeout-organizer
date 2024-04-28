rm -rf Takeout
cp -r Takeout.bak Takeout
rm -rf target
mkdir target

go build
./google-photos-takeout-organizer Takeout target
