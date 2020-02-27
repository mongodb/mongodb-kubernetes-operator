package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
)

func main() {
	if err := downloadFile(mustMakeOptions()); err != nil {
		panic(fmt.Errorf("error downloading file: %s", err))
	}
}

type downloadOptions struct {
	url, fileName, bindir string
	perms                 os.FileMode
}

func mustMakeOptions() downloadOptions {
	perms := os.Getenv("PERMISSIONS")
	intPerms, err := strconv.Atoi(perms)
	if err != nil {
		panic(err)
	}
	return downloadOptions{
		url:      os.Getenv("URL"),
		fileName: os.Getenv("FILENAME"),
		perms:    os.FileMode(intPerms),
		bindir:   os.Getenv("BINDIR"),
	}
}

func downloadFile(opts downloadOptions) error {
	fmt.Printf("Using download options: %+v\n", opts)
	fullPath := path.Join(opts.bindir, opts.fileName)
	fmt.Printf("full path to directory: %s\n", fullPath)
	if err := os.MkdirAll(opts.bindir, opts.perms); err != nil {
		return fmt.Errorf("error making directory %s with permissions %d: %s", opts.bindir, opts.perms, err)
	}
	if err := fetchFile(fullPath, opts.url); err != nil {
		return fmt.Errorf("error fetching file: %s", err)
	}
	fmt.Printf("successfully downloaded file from %s to %s\n", opts.url, fullPath)
	if err := os.Chmod(fullPath, opts.perms); err != nil {
		return fmt.Errorf("error changing file permissions: %s", err)
	}
	return nil
}

func fetchFile(filePath, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error getting url: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %s", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
