package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"upspin.io/access"
	"upspin.io/bind"
	upath "upspin.io/path"
	"upspin.io/upspin"
)

// MakeDirs recursively creates directories in p if they don't exist.
func MakeDirs(cl upspin.Client, p upspin.PathName) error {
	_, err := cl.Lookup(p, false)
	if err == nil {
		return nil
	}

	dir := ""
	for i, d := range strings.Split(string(p), "/") {
		if d == "" {
			continue
		} else if i > 0 {
			dir = dir + "/"
		}
		dir += d
		cl.MakeDirectory(upspin.PathName(dir))
	}

	_, err = cl.Lookup(p, false)
	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	return nil
}

func recursiveList(cl upspin.Client, p upspin.PathName) ([]*upspin.DirEntry, error) {
	ents, err := cl.Glob(string(Join(p, "*")))
	if err != nil {
		return nil, err
	}

	files := []*upspin.DirEntry{}
	for _, ent := range ents {
		if path.Base(string(ent.SignedName)) == "Access" {
			continue
		}

		if ent.IsDir() {
			subfiles, err := recursiveList(cl, ent.SignedName)
			if err != nil {
				return nil, err
			}
			files = append(files, subfiles...)
		} else {
			files = append(files, ent)
		}
	}
	return files, nil
}

func Copy(cl upspin.Client, src, dst upspin.PathName) (err error) {
	_, err = cl.Lookup(dst, false)
	if err == nil {
		return fmt.Errorf("copy destination '%v' already exists", dst)
	}

	if path.Base(string(src)) == "Access" {
		return fmt.Errorf("cannot copy 'Access' files")
	}

	if err := MakeDirs(cl, upspin.PathName(path.Dir(string(dst)))); err != nil {
		return err
	}

	sf, err := cl.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := sf.Close(); err == nil {
			err = err2
		}
	}()

	df, err := cl.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := df.Close(); err == nil {
			err = err2
		}
	}()

	_, err = io.Copy(df, sf)
	return err
}

func Synchronize(cl upspin.Client, src, dst upspin.PathName) error {
	srcs, err := recursiveList(cl, src)
	if err != nil {
		return fmt.Errorf("failed to retrieve src files: %v", err)
	}

	pdst, err := upath.Parse(dst)
	if err != nil {
		return err
	}

	for _, ent := range srcs {
		p, err := upath.Parse(ent.SignedName)
		if err != nil {
			return err
		}
		srcpath := ent.SignedName
		dstpath := Join(upspin.PathName(pdst.User()), p.FilePath())
		_, err = cl.Lookup(dstpath, false)
		if err == nil {
			continue // file exists at destination already
		}
		err = Copy(cl, srcpath, dstpath)
		if err != nil {
			return err
		}
	}
	return nil
}

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

// Join builds an upspin path for the given upspin path and the passed path elements joined
// together.
func Join(u upspin.PathName, paths ...string) upspin.PathName {
	return upspin.PathName(path.Join(append([]string{string(u)}, paths...)...))
}

func readAccess(cl upspin.Client, dir upspin.PathName) (*access.Access, error) {
	pth := Join(dir, "Access")
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

type LocalFile struct {
	*os.File
}

func (lf LocalFile) Name() upspin.PathName { return upspin.PathName(lf.File.Name()) }

type LocalClient struct {
}

func (lc *LocalClient) Get(name upspin.PathName) ([]byte, error) {
	return ioutil.ReadFile(string(name))
}
func (lc *LocalClient) Lookup(name upspin.PathName, followFinal bool) (*upspin.DirEntry, error) {
	panic("unimplemented")
}
func (lc *LocalClient) Put(name upspin.PathName, data []byte) (*upspin.DirEntry, error) {
	err := ioutil.WriteFile(string(name), data, 0644)
	return &upspin.DirEntry{SignedName: name, Name: name}, err
}

func (lc *LocalClient) PutLink(oldName, newName upspin.PathName) (*upspin.DirEntry, error) {
	panic("unimplemented")
}
func (lc *LocalClient) PutDuplicate(oldName, newName upspin.PathName) (*upspin.DirEntry, error) {
	panic("unimplemented")
}
func (lc *LocalClient) MakeDirectory(dirName upspin.PathName) (*upspin.DirEntry, error) {
	err := os.MkdirAll(string(dirName), 0755)
	return &upspin.DirEntry{SignedName: dirName, Name: dirName}, err
}
func (lc *LocalClient) Rename(oldName, newName upspin.PathName) error {
	panic("unimplemented")
}
func (lc *LocalClient) Delete(name upspin.PathName) error {
	panic("unimplemented")
}
func (lc *LocalClient) Glob(pattern string) ([]*upspin.DirEntry, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var dirs []*upspin.DirEntry
	for _, m := range matches {
		d := &upspin.DirEntry{
			SignedName: upspin.PathName(m),
			Name:       upspin.PathName(m),
		}
		dirs = append(dirs, d)
	}
	return dirs, nil
}

func (lc *LocalClient) Create(name upspin.PathName) (upspin.File, error) {
	f, err := os.Create(string(name))
	return LocalFile{f}, err
}
func (lc *LocalClient) Open(name upspin.PathName) (upspin.File, error) {
	f, err := os.Open(string(name))
	return LocalFile{f}, err
}
func (lc *LocalClient) DirServer(name upspin.PathName) (upspin.DirServer, error) {
	panic("unimplemented")
}
