package main

type Implementation interface {
	SetFlag(path string) error
	RemoveFlag(path string) error
	HasFlag(path string) (bool, error)
}

func SetDropboxIgnoreFlag(path string) error {
	return implementation.SetFlag(path)
}

func RemoveDropboxIgnoreFlag(path string) error {
	return implementation.RemoveFlag(path)
}

func HasDropboxIgnoreFlag(path string) (bool, error) {
	return implementation.HasFlag(path)
}
