package main

import (
	"io"
	"path"

	"upspin.io/access"
	"upspin.io/bind"
	"upspin.io/upspin"
)

func AddFile(cl upspin.Client, fpath upspin.PathName, r io.Reader) (err error) {
	f, err := cl.Create(fpath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := f.Close(); err == nil {
			err = err2
		}
	}()

	_, err = io.Copy(f, r)
	return err
}

// join builds an upspin path for the given upspin path and the passed path elements joined
// together.
func join(u upspin.PathName, paths ...string) upspin.PathName {
	return upspin.PathName(path.Join(append([]string{string(u)}, paths...)...))
}

func readAccess(cl upspin.Client, dir upspin.PathName) (*access.Access, error) {
	pth := join(dir, "Access")
	data, err := cl.Get(pth)
	if err != nil {
		return nil, err
	}
	return access.Parse(pth, data)
}

// lookup returns the public key for a given upspin user using the key server
// endpoint contained in the given upspin config.
func lookup(config upspin.Config, name upspin.UserName) (key upspin.PublicKey, err error) {
	keyserv, err := bind.KeyServer(config, config.KeyEndpoint())
	if err != nil {
		return key, err
	}
	user, err := keyserv.Lookup(name)
	if err != nil {
		return key, err
	}
	return user.PublicKey, nil
}
