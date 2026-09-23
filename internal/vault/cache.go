package vault

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/atomicfile"
)

type cacheHeader struct {
	Model         string
	Digest        string
	Preprocessing int
}

type cacheFile struct {
	Header    cacheHeader
	Dimension int
	Vectors   map[string][][]float32
}

func DefaultCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.FromSlash(cacheDirName)), nil
}

// loadCache answers the vectors cached for header. Anything unreadable, built
// for another model or preprocessing, or of mixed dimension is a miss.
func loadCache(path string, header cacheHeader) map[string][][]float32 {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string][][]float32{}
	}
	var file cacheFile
	if gob.NewDecoder(bytes.NewReader(b)).Decode(&file) != nil || file.Header != header || file.Vectors == nil {
		return map[string][][]float32{}
	}
	for _, vectors := range file.Vectors {
		for _, v := range vectors {
			if len(v) != file.Dimension {
				return map[string][][]float32{}
			}
		}
	}
	return file.Vectors
}

func encodeCache(header cacheHeader, vectors map[string][][]float32) ([]byte, error) {
	file := cacheFile{Header: header, Vectors: vectors}
	for _, vs := range vectors {
		if len(vs) > 0 {
			file.Dimension = len(vs[0])
			break
		}
	}
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(file)
	return b.Bytes(), err
}

func writeCache(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), cacheDirMode); err != nil {
		return err
	}
	return atomicfile.Replace(path, b, cacheMode)
}
