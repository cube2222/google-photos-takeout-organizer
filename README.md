# Google Photos Takeout Organizer

## Disclaimer
This is a quick'n'dirty script I wrote without much testing to organize my own Google Photos takeout data. Use at your own risk.

https://github.com/TheLastGimbus/GooglePhotosTakeoutHelper looks like a much more polished project, but it didn't deduplicate correctly for me. I also wanted a slightly different approach to album organization.

## Overview
This is a little script to organize Google Photos takeout data. Here's the target directory structure:
```
-- targetDirectory
   |-- Album-only Photos
   |-- Albums
       |-- Album1
       |-- Album2
       |-- ...
   |-- Archive
   |-- Photos
```

The Photos directory contains all the photos from your core photos collection.

The Album-only Photos directory contains all the photos that are in an album (or a sharing conversation) but not in your core Photos collection.

The Albums directory contains subdirectories for each album and conversation. Each of these subdirectories contains symlinks to photos in the previous two directories.

The Archive directory contains all the photos from your Archive.

Importantly, the Trash is ignored.

Photos are deduplicated based on a hash of their content. The Archive does not take part in this deduplication.

## EXIF data

The script will attempt to run `exiftool` to update file modified times to match the photo's EXIF metadata creation times. You can install it e.g. via homebrew:
```
brew install exiftool
```

## Usage
Just build and run the script passing as the first argument the extracted `Takeout` directory, and as the second argument the target directory where the organized data will be stored.

```bash
~> go build
~> ./google-photos-takeout-organizer /path/to/Takeout /path/to/targetDirectory
...
```
