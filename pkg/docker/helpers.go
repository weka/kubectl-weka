package docker

import (
	"fmt"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/utils"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type progressState struct {
	logger *logging.Logger

	direction string
	total     int64
	current   atomic.Int64

	mu         sync.Mutex
	blobTotals map[string]int64
	blobSeen   map[string]int64
	lastRender time.Time
	imageName  string
}

func newProgressState(logger *logging.Logger, direction string, total int64) *progressState {
	return &progressState{
		logger:     logger,
		direction:  direction,
		total:      total,
		blobTotals: map[string]int64{},
		blobSeen:   map[string]int64{},
	}
}

func (p *progressState) registerBlob(digest string, size int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.blobTotals[digest]; !exists {
		p.blobTotals[digest] = size
		p.logger.Debug("blob scheduled",
			"direction", p.direction,
			"digest", digest,
			"size", size,
		)
	}
}

func (p *progressState) setImageName(imageName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.imageName = imageName
}

func (p *progressState) advanceBlob(digest string, delta int64) {
	if delta <= 0 {
		return
	}

	p.current.Add(delta)

	p.mu.Lock()
	p.blobSeen[digest] += delta
	now := time.Now()
	shouldRender := now.Sub(p.lastRender) >= 200*time.Millisecond
	if shouldRender {
		p.lastRender = now
	}
	p.mu.Unlock()

	if shouldRender {
		p.renderLine()
	}
}

func (p *progressState) completeBlob(digest string) {
	p.mu.Lock()
	seen := p.blobSeen[digest]
	total := p.blobTotals[digest]
	p.blobSeen[digest] = total // hack to show 100%
	p.mu.Unlock()

	p.renderLine()

	p.logger.Debug("blob completed",
		"direction", p.direction,
		"digest", digest,
		"bytes", seen,
		"expected", total,
	)
}

func (p *progressState) renderLine() {
	total := p.total
	current := p.current.Load()

	p.mu.Lock()
	imageName := p.imageName
	p.mu.Unlock()

	if total <= 0 {
		fmt.Printf("\r%s progress: %s [%s]", p.direction, utils.HumanBytes(current), imageName)
		return
	}

	const width = 30
	percent := float64(current) / float64(total)
	if percent > 1 {
		percent = 1
	}

	filled := int(percent * width)
	if current > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)
	fmt.Printf(
		"\r%-10s [%s] %6.2f%% (%s/%s) [%s]",
		p.direction,
		bar,
		percent*100,
		utils.HumanBytes(current),
		utils.HumanBytes(total),
		imageName,
	)
}

func (p *progressState) addBytes(bytes int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current.Add(bytes)
}

func (p *progressState) finish() {
	p.current.Store(p.total) // hack to always show 100%
	p.renderLine()
	fmt.Println()
}

type progressReadCloser struct {
	rc        io.ReadCloser
	onRead    func(int64)
	onClose   func()
	closeOnce sync.Once
}

func (p *progressReadCloser) Read(b []byte) (int, error) {
	n, err := p.rc.Read(b)
	if n > 0 && p.onRead != nil {
		p.onRead(int64(n))
	}
	if err == io.EOF {
		p.closeOnce.Do(func() {
			if p.onClose != nil {
				p.onClose()
			}
		})
	}
	return n, err
}

func (p *progressReadCloser) Close() error {
	p.closeOnce.Do(func() {
		if p.onClose != nil {
			p.onClose()
		}
	})
	return p.rc.Close()
}

func imageTotalBlobSize(img v1.Image) (int64, error) {
	layers, err := img.Layers()
	if err != nil {
		return 0, err
	}

	var total int64

	cfgName, err := img.ConfigName()
	if err == nil {
		// Config blob size is not directly exposed, but we can read raw config file.
		rawCfg, err := img.RawConfigFile()
		if err == nil {
			_ = cfgName
			total += int64(len(rawCfg))
		}
	}

	for _, l := range layers {
		sz, err := l.Size()
		if err != nil {
			return 0, err
		}
		total += sz
	}
	return total, nil
}

func indexTotalBlobSize(idx v1.ImageIndex, wantArch map[string]struct{}, wantOS string) (int64, error) {
	im, err := idx.IndexManifest()
	if err != nil {
		return 0, err
	}

	var total int64
	seenArch := map[string]struct{}{}

	for _, m := range im.Manifests {
		if m.Platform == nil {
			continue
		}
		arch := utils.NormalizeValue(m.Platform.Architecture)
		osName := utils.NormalizeValue(m.Platform.OS)

		if !matchesArch(arch, wantArch) || !matchesOS(osName, wantOS) {
			continue
		}
		if _, exists := seenArch[arch]; exists {
			continue
		}

		img, err := idx.Image(m.Digest)
		if err != nil {
			return 0, err
		}
		sz, err := imageTotalBlobSize(img)
		if err != nil {
			return 0, err
		}
		total += sz
		seenArch[arch] = struct{}{}
	}

	return total, nil
}

func ociLayoutTotalBlobSize(layoutDir string) (int64, error) {
	var total int64
	blobsDir := filepath.Join(layoutDir, "blobs")

	err := filepath.Walk(blobsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}
