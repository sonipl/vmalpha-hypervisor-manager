package handlers

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/sys/unix"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const browserUploadLimit int64 = 512 << 20

func browserPath(p string) (string, error) {
	if p == "" {
		return ".", nil
	}
	if strings.ContainsAny(p, "\x00\\") || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("relative datastore path required")
	}
	for _, v := range strings.Split(p, "/") {
		if v == ".." {
			return "", fmt.Errorf("parent traversal forbidden")
		}
	}
	return path.Clean(p), nil
}

// os.Root keeps every filesystem operation within an already-opened datastore,
// including concurrent symlink replacement. No caller supplies an absolute root.
func (h *StorageBackendsHandler) browserRoot(c *gin.Context) (*os.Root, string, error) {
	p, err := browserPath(c.Query("path"))
	if err != nil {
		return nil, "", err
	}
	b, _, ok := backendByID(loadStorageConfig(), c.Param("id"))
	if !ok {
		return nil, "", fmt.Errorf("datastore not found")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err = externalMountIdentity(ctx, b); err != nil {
		return nil, "", err
	}
	target, _, _, _ := browserMountConfig(b)
	root, err := os.OpenRoot(target)
	if err != nil {
		return nil, "", err
	}
	// Validate the opened descriptor, closing the mount-check/open race. An
	// unmounted datastore must never fall back to Manager local disk writes.
	dir, e := root.Open(".")
	if e != nil {
		root.Close()
		return nil, "", fmt.Errorf("mount root unavailable")
	}
	var fs unix.Statfs_t
	e = unix.Fstatfs(int(dir.Fd()), &fs)
	dir.Close()
	if e != nil || fs.Type != 0x6969 {
		root.Close()
		return nil, "", fmt.Errorf("opened datastore is not NFS")
	}
	// Reject symlinks in the selected path; Root also prevents escape during races.
	current := "."
	for _, part := range strings.Split(p, "/") {
		current = path.Join(current, part)
		fi, e := root.Lstat(current)
		if os.IsNotExist(e) {
			break
		}
		if e != nil || fi.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, "", fmt.Errorf("symlink paths unsupported")
		}
	}
	return root, p, nil
}
func (h *StorageBackendsHandler) BrowseDatastore(c *gin.Context) {
	root, p, err := h.browserRoot(c)
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	defer root.Close()
	f, err := root.Open(p)
	if err != nil {
		c.JSON(404, gin.H{"error": "directory unavailable"})
		return
	}
	defer f.Close()
	entries, err := f.ReadDir(1001)
	if err != nil && err != io.EOF {
		c.JSON(400, gin.H{"error": "not a readable directory"})
		return
	}
	if len(entries) > 1000 {
		c.JSON(413, gin.H{"error": "directory exceeds 1000 entries"})
		return
	}
	rows := []gin.H{}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if !fi.IsDir() && !fi.Mode().IsRegular() {
			continue
		}
		rows = append(rows, gin.H{"name": e.Name(), "directory": e.IsDir(), "size": fi.Size(), "modified_at": fi.ModTime()})
	}
	c.JSON(200, gin.H{"path": p, "entries": rows})
}
func (h *StorageBackendsHandler) DownloadDatastore(c *gin.Context) {
	root, p, err := h.browserRoot(c)
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	defer root.Close()
	f, err := root.Open(p)
	if err != nil {
		c.JSON(404, gin.H{"error": "file unavailable"})
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		c.JSON(400, gin.H{"error": "regular file required"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(fi.Name())))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", "application/octet-stream")
	http.ServeContent(c.Writer, c.Request, fi.Name(), fi.ModTime(), f)
}
func (h *StorageBackendsHandler) MkdirDatastore(c *gin.Context) {
	root, p, err := h.browserRoot(c)
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	defer root.Close()
	if p == "." {
		c.JSON(400, gin.H{"error": "root cannot be created"})
		return
	}
	if root.Mkdir(p, 0750) != nil {
		c.JSON(409, gin.H{"error": "folder exists or parent unavailable"})
		return
	}
	c.JSON(201, gin.H{"path": p})
}
func (h *StorageBackendsHandler) DeleteDatastoreEntry(c *gin.Context) {
	root, p, err := h.browserRoot(c)
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	defer root.Close()
	if p == "." {
		c.JSON(400, gin.H{"error": "datastore root cannot be deleted"})
		return
	}
	if root.Remove(p) != nil {
		c.JSON(409, gin.H{"error": "remove failed; directories must be empty"})
		return
	}
	c.Status(204)
}
func (h *StorageBackendsHandler) UploadDatastore(c *gin.Context) {
	root, p, err := h.browserRoot(c)
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	defer root.Close()
	if p == "." {
		c.JSON(400, gin.H{"error": "destination filename required"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, browserUploadLimit)
	defer c.Request.Body.Close()
	// Exclusive creation prevents silently replacing VM disks or existing files.
	f, err := root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if err != nil {
		c.JSON(409, gin.H{"error": "destination exists or parent unavailable"})
		return
	}
	_, err = io.Copy(f, c.Request.Body)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = root.Remove(p)
		c.JSON(400, gin.H{"error": "upload failed or exceeds 512 MiB"})
		return
	}
	c.JSON(201, gin.H{"path": p})
}
