package vault

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strings"
	"syscall"
	"unicode"
)

type indexedChunk struct {
	ArtifactID string `json:"artifactId"`
	SourceHash string `json:"sourceHash"`
	Chunk
}
type posting struct {
	Chunk     int `json:"chunk"`
	Frequency int `json:"frequency"`
}
type generation struct {
	Excluded    map[string]string       `json:"excluded"`
	Version     int                     `json:"version"`
	Name        string                  `json:"name"`
	Chunker     string                  `json:"chunker"`
	Fingerprint Fingerprint             `json:"fingerprint"`
	Sources     map[string]string       `json:"sources"`
	Hashes      map[string]string       `json:"hashes"`
	Documents   map[string]Document     `json:"documents"`
	Lineage     map[string]LineageState `json:"lineage"`
	Chunks      []indexedChunk          `json:"chunks"`
	Postings    map[string][]posting    `json:"postings"`
	Lengths     []int                   `json:"lengths"`
	VectorHash  string                  `json:"vectorHash"`
	Vectors     []float32               `json:"-"`
}

func readRegular(root *os.Root, path string, limit int64) ([]byte, error) {
	file, err := root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, ErrIndexCorrupt
	}
	b, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, ErrIndexCorrupt
	}
	return b, nil
}
func randomName(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
func replaceRoot(root *os.Root, path string, b []byte) error {
	if info, err := root.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return ErrIndexState
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	temporary, err := randomName(".replace-")
	if err != nil {
		return err
	}
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if _, err = file.Write(b); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = root.Rename(temporary, path); err != nil {
		return err
	}
	return syncRoot(root)
}
func syncRoot(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func openCache(profile Profile, installationID string) (*os.Root, error) {
	if !componentPattern.MatchString(installationID) {
		return nil, ErrIndexState
	}
	vaultRoot, err := os.OpenRoot(profile.Root)
	if err != nil {
		return nil, ErrIndexState
	}
	defer vaultRoot.Close()
	current := vaultRoot
	for _, component := range []string{".qrouton", "index", installationID} {
		next, err := openChildDirectory(current, component, true)
		if current != vaultRoot {
			current.Close()
		}
		if err != nil {
			return nil, ErrIndexState
		}
		current = next
	}
	return current, nil
}
func cleanupStaging(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".staging-") {
			if err := root.RemoveAll(entry.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}
func validFingerprint(fp Fingerprint) bool {
	return fp.Model != "" && fp.Digest != "" && fp.Tokenizer == NewTokenizer().Checksum() && fp.Preprocessing == PreprocessingVersion && fp.Dimensions == EmbeddingDimensions && fp.MaxTokens == EmbeddingMaxTokens
}
func lexicalTerms(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}
func postingsFor(chunks []indexedChunk) (map[string][]posting, []int) {
	postings := map[string][]posting{}
	lengths := make([]int, len(chunks))
	for index, chunk := range chunks {
		counts := map[string]int{}
		terms := lexicalTerms(chunk.Input)
		lengths[index] = len(terms)
		for _, term := range terms {
			counts[term]++
		}
		for term, count := range counts {
			postings[term] = append(postings[term], posting{Chunk: index, Frequency: count})
		}
	}
	return postings, lengths
}
func validateGeneration(g *generation) error {
	if g.Version != indexSchemaVersion || g.Chunker != ChunkerVersion || !validFingerprint(g.Fingerprint) || !strings.HasPrefix(g.Name, "g-") || !componentPattern.MatchString(g.Name) {
		return ErrIndexCorrupt
	}
	if g.Sources == nil || g.Documents == nil || g.Excluded == nil || g.Hashes == nil || g.Lineage == nil || len(g.Vectors) != len(g.Chunks)*EmbeddingDimensions {
		return ErrIndexCorrupt
	}
	for path, hash := range g.Sources {
		if path == "" || len(hash) != 64 {
			return ErrIndexCorrupt
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return ErrIndexCorrupt
		}
	}
	for id, document := range g.Documents {
		if id != document.ID || Validate(document) != nil {
			return ErrIndexCorrupt
		}
	}
	for index, chunk := range g.Chunks {
		document, exists := g.Documents[chunk.ArtifactID]
		state, known := g.Lineage[chunk.ArtifactID]
		if !exists || !known || state.Invalid || state.EffectiveState != "active" || document.Kind == "dead_end" || chunk.StartLine < 1 || chunk.EndLine < chunk.StartLine || chunk.Input == "" || NewTokenizer().Count(chunk.Input) > EmbeddingMaxTokens {
			return ErrIndexCorrupt
		}
		var norm float64
		for _, value := range g.Vectors[index*EmbeddingDimensions : (index+1)*EmbeddingDimensions] {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return ErrIndexCorrupt
			}
			norm += float64(value) * float64(value)
		}
		if math.Abs(norm-1) > 0.001 {
			return ErrIndexCorrupt
		}
	}
	postings, lengths := postingsFor(g.Chunks)
	if !reflect.DeepEqual(postings, g.Postings) || !reflect.DeepEqual(lengths, g.Lengths) {
		return ErrIndexCorrupt
	}
	return nil
}
func writeGeneration(root *os.Root, g *generation) error {
	if err := validateGeneration(g); err != nil {
		return err
	}
	staging, err := randomName(".staging-")
	if err != nil {
		return err
	}
	if err := root.Mkdir(staging, 0700); err != nil {
		return err
	}
	defer root.RemoveAll(staging)
	stage, err := root.OpenRoot(staging)
	if err != nil {
		return err
	}
	defer stage.Close()
	vectors := make([]byte, len(g.Vectors)*4)
	for i, value := range g.Vectors {
		binary.LittleEndian.PutUint32(vectors[i*4:], math.Float32bits(value))
	}
	g.VectorHash = contentHash(string(vectors))
	manifest, err := json.Marshal(g)
	if err != nil {
		return err
	}
	if err := replaceRoot(stage, generationVectors, vectors); err != nil {
		return err
	}
	if err := replaceRoot(stage, generationManifest, manifest); err != nil {
		return err
	}
	if _, err := readGeneration(stage); err != nil {
		return err
	}
	if err := root.Rename(staging, g.Name); err != nil {
		return err
	}
	if err := syncRoot(root); err != nil {
		return err
	}
	return replaceRoot(root, currentFilename, []byte(g.Name))
}
func readGeneration(root *os.Root) (*generation, error) {
	manifest, err := readRegular(root, generationManifest, maxIndexBytes)
	if err != nil {
		return nil, ErrIndexCorrupt
	}
	var g generation
	if err := json.Unmarshal(manifest, &g); err != nil {
		return nil, ErrIndexCorrupt
	}
	vectors, err := readRegular(root, generationVectors, maxIndexBytes)
	if err != nil || len(vectors)%4 != 0 || contentHash(string(vectors)) != g.VectorHash {
		return nil, ErrIndexCorrupt
	}
	g.Vectors = make([]float32, len(vectors)/4)
	for i := range g.Vectors {
		g.Vectors[i] = math.Float32frombits(binary.LittleEndian.Uint32(vectors[i*4:]))
	}
	if err := validateGeneration(&g); err != nil {
		return nil, err
	}
	return &g, nil
}
func loadGeneration(root *os.Root) (*generation, error) {
	current, err := readRegular(root, currentFilename, 128)
	if err != nil {
		return nil, err
	}
	name := string(current)
	if !strings.HasPrefix(name, "g-") || !componentPattern.MatchString(name) {
		return nil, ErrIndexCorrupt
	}
	directory, err := openChildDirectory(root, name, false)
	if err != nil {
		return nil, ErrIndexCorrupt
	}
	defer directory.Close()
	g, err := readGeneration(directory)
	if err != nil {
		return nil, err
	}
	if g.Name != name {
		return nil, ErrIndexCorrupt
	}
	return g, nil
}
func bodyFirstLine(result ReadResult) int {
	normalized := strings.ReplaceAll(result.Content, "\r\n", "\n")
	offset := strings.Index(normalized[4:], "\n---\n")
	if offset < 0 {
		return 1
	}
	return strings.Count(normalized[:4+offset+5], "\n") + 1
}
func generationName() (string, error) {
	name, err := randomName("g-")
	if err != nil {
		return "", fmt.Errorf("%w: generation identity", ErrIndexState)
	}
	return name, nil
}

func openChildDirectory(root *os.Root, name string, create bool) (*os.Root, error) {
	if create {
		if err := root.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}
	beforePath, err := root.Lstat(name)
	if err != nil || !beforePath.IsDir() || beforePath.Mode()&os.ModeSymlink != 0 {
		return nil, ErrIndexState
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !os.SameFile(beforePath, before) {
		return nil, ErrIndexState
	}
	child, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		child.Close()
		return nil, ErrIndexState
	}
	return child, nil
}
func pruneGenerations(root *os.Root, current, previous string) {
	file, err := root.Open(".")
	if err != nil {
		return
	}
	defer file.Close()
	entries, err := file.ReadDir(-1)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == current || name == previous || !entry.IsDir() || !strings.HasPrefix(name, "g-") {
			continue
		}
		child, err := openChildDirectory(root, name, false)
		if err != nil {
			continue
		}
		g, err := readGeneration(child)
		child.Close()
		if err == nil && g.Name == name {
			_ = root.RemoveAll(name)
		}
	}
}
