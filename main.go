package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var photosForYearRegex = regexp.MustCompile(`Photos from (\d{4})`)

func main() {
	sourceDir := os.Args[1]
	targetDir := os.Args[2]

	if err := run(sourceDir, targetDir); err != nil {
		log.Fatal(err)
	}
}

func run(sourceDir, targetDir string) error {
	sourceGooglePhotosDir := filepath.Join(sourceDir, "Google Photos")

	googlePhotosDirElements, err := os.ReadDir(sourceGooglePhotosDir)
	if err != nil {
		return fmt.Errorf("failed to read google photos directory %s: %w", sourceGooglePhotosDir, err)
	}

	photosIndex := make(map[string]string)

	// TODO: Parallelize when bored.
	log.Println("Moving and indexing main-directory photos...")
	targetPhotosDir := filepath.Join(targetDir, "Photos")
	{ // Move main-directory photos to target directory and index by sha.
		if err := os.MkdirAll(targetPhotosDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create target photos directory %s: %w", targetPhotosDir, err)
		}

		for _, googlePhotosDirElement := range googlePhotosDirElements {
			if !googlePhotosDirElement.IsDir() {
				continue
			}

			if !photosForYearRegex.MatchString(googlePhotosDirElement.Name()) {
				continue
			}

			sourcePhotosForYearDir := filepath.Join(sourceGooglePhotosDir, googlePhotosDirElement.Name())

			sourcePhotosForYear, err := os.ReadDir(sourcePhotosForYearDir)
			if err != nil {
				return fmt.Errorf("failed to read photos directory %s: %w", googlePhotosDirElement.Name(), err)
			}

			for _, sourcePhoto := range sourcePhotosForYear {
				if filepath.Ext(sourcePhoto.Name()) == ".json" {
					continue
				}

				sourcePath := filepath.Join(sourcePhotosForYearDir, sourcePhoto.Name())
				targetPath := filepath.Join(targetPhotosDir, sourcePhoto.Name())

				sha, err := getFileSha(sourcePath)
				if err != nil {
					return fmt.Errorf("failed to get sha for file %s: %w", targetPath, err)
				}

				targetPath, err = movePhotoIntoPlace(sourcePath, targetPath, sha)
				if err != nil {
					return fmt.Errorf("failed to move photo %s: %w", sourcePhoto.Name(), err)
				}

				photosIndex[sha] = targetPath
			}
		}
	}

	log.Println("Moving archive...")
	targetArchiveDir := filepath.Join(targetDir, "Archive")
	sourceArchiveDir := filepath.Join(sourceGooglePhotosDir, "Archive")
	{
		if err := os.MkdirAll(targetArchiveDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create archive directory: %w", err)
		}

		sourceArchivePhotos, err := os.ReadDir(filepath.Join(sourceGooglePhotosDir, "Archive"))
		if err != nil {
			return fmt.Errorf("failed to read archive photos directory: %w", err)
		}

		for _, sourceArchivePhoto := range sourceArchivePhotos {
			if filepath.Ext(sourceArchivePhoto.Name()) == ".json" {
				continue
			}

			sourcePath := filepath.Join(sourceArchiveDir, sourceArchivePhoto.Name())
			targetPath := filepath.Join(targetArchiveDir, sourceArchivePhoto.Name())

			sha, err := getFileSha(sourcePath)
			if err != nil {
				return fmt.Errorf("failed to get sha for file %s: %w", targetPath, err)
			}

			targetPath, err = movePhotoIntoPlace(sourcePath, targetPath, sha)
			if err != nil {
				return fmt.Errorf("failed to move archive photo %s: %w", sourceArchivePhoto.Name(), err)
			}

			// Just in case someone has a duplicate photo in the photos directory and in the archive.
			if _, ok := photosIndex[sha]; !ok {
				photosIndex[sha] = targetPath
			}
		}
	}

	log.Println("Moving albums...")
	targetAlbumOnlyPhotosDir := filepath.Join(targetDir, "Album-only Photos")
	{
		// Move albums to target directory.
		// Duplicates from the main photos directory become symlinks to the photos file.
		// Non-duplicates become symlinks to a special "Album-only Photos" directory.
		if err := os.MkdirAll(targetAlbumOnlyPhotosDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create album-only photos directory: %w", err)
		}

		targetAlbumsDir := filepath.Join(targetDir, "Albums")
		if err := os.MkdirAll(targetAlbumsDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create albums directory: %w", err)
		}

		for _, googlePhotosDirElement := range googlePhotosDirElements {
			if !googlePhotosDirElement.IsDir() {
				continue
			}
			if photosForYearRegex.MatchString(googlePhotosDirElement.Name()) {
				continue
			}
			if googlePhotosDirElement.Name() == "Trash" {
				continue
			}
			if googlePhotosDirElement.Name() == "Archive" {
				continue
			}

			sourceAlbumDir := filepath.Join(sourceGooglePhotosDir, googlePhotosDirElement.Name())
			albumPhotos, err := os.ReadDir(sourceAlbumDir)
			if err != nil {
				return fmt.Errorf("failed to read album photos directory %s: %w", googlePhotosDirElement.Name(), err)
			}

			targetAlbumDir := filepath.Join(targetAlbumsDir, googlePhotosDirElement.Name())
			if err := os.MkdirAll(targetAlbumDir, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create album directory %s: %w", targetAlbumDir, err)
			}

			for _, albumPhoto := range albumPhotos {
				if filepath.Ext(albumPhoto.Name()) == ".json" {
					continue
				}

				sourcePath := filepath.Join(sourceAlbumDir, albumPhoto.Name())
				targetPath := filepath.Join(targetAlbumDir, albumPhoto.Name())

				sha, err := getFileSha(sourcePath)
				if err != nil {
					return fmt.Errorf("failed to get sha for file %s: %w", sourcePath, err)
				}

				var actualPhotoPath string
				if mainDirectoryPhoto, ok := photosIndex[sha]; ok {
					actualPhotoPath = mainDirectoryPhoto
				} else {
					albumOnlyPhotoPath := filepath.Join(targetAlbumOnlyPhotosDir, albumPhoto.Name())
					albumOnlyPhotoPath, err = movePhotoIntoPlace(sourcePath, albumOnlyPhotoPath, sha)
					if err != nil {
						return fmt.Errorf("failed to move album-only photo %s: %w", albumPhoto.Name(), err)
					}

					photosIndex[sha] = albumOnlyPhotoPath
					actualPhotoPath = albumOnlyPhotoPath
				}

				targetRelativePhotoPath, err := filepath.Rel(filepath.Dir(targetPath), actualPhotoPath)
				if err != nil {
					return fmt.Errorf("failed to get relative path for album photo %s: %w", albumPhoto.Name(), err)
				}

				if err := os.Symlink(targetRelativePhotoPath, targetPath); err != nil {
					return fmt.Errorf("failed to create symlink for album photo %s: %w", albumPhoto.Name(), err)
				}
			}
		}
	}

	{ // Run exiftool on all photos to update file modified date based on exif creation date.
		log.Println("Updating file modified date based on exif creation date via exiftool...")
		for _, dir := range []string{targetPhotosDir, targetArchiveDir, targetAlbumOnlyPhotosDir} {
			cmd := exec.Command("exiftool", "-FileModifyDate<CreateDate", "-ext", "*", "-r", dir)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to run exiftool: %w", err)
			}
		}
		log.Println("Updating completed.")
	}

	return nil
}

func getFileSha(file string) (string, error) {
	hash := sha1.New()
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", file, err)
	}
	defer f.Close()

	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("failed to hash file %s: %w", file, err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func movePhotoIntoPlace(sourcePath, targetPath, sha string) (string, error) {
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		ext := filepath.Ext(targetPath)
		targetPath = fmt.Sprintf("%s_%s%s", targetPath[:len(targetPath)-len(ext)], sha, ext)
	}

	if err := os.Rename(sourcePath, targetPath); err != nil {
		return "", fmt.Errorf("failed to move photo %s to %s: %w", sourcePath, targetPath, err)
	}

	return targetPath, nil
}
