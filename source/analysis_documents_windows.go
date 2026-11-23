package main

import "golang.org/x/sys/windows"

func analysisDocuments() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
}
