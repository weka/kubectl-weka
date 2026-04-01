package docker

import (
	"context"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"io"
)

type progressLayer struct {
	v1.Layer
	digest   string
	progress *progressState
}

func (l *progressLayer) Compressed() (io.ReadCloser, error) {
	rc, err := l.Layer.Compressed()
	if err != nil {
		return nil, err
	}

	if l.progress != nil {
		size, _ := l.Layer.Size()
		if size > 0 {
			l.progress.registerBlob(l.digest, size)
		}
	}

	return &progressReadCloser{
		rc: rc,
		onRead: func(n int64) {
			if l.progress != nil {
				l.progress.advanceBlob(l.digest, n)
			}
		},
		onClose: func() {
			if l.progress != nil {
				l.progress.completeBlob(l.digest)
			}
		},
	}, nil
}

type progressImage struct {
	v1.Image
	progress *progressState

	layersByDigest map[string]v1.Layer
	layers         []v1.Layer
}

func wrapImageWithProgress(img v1.Image, progress *progressState) (v1.Image, error) {
	origLayers, err := img.Layers()
	if err != nil {
		return nil, err
	}

	wrapped := make([]v1.Layer, 0, len(origLayers))
	byDigest := make(map[string]v1.Layer, len(origLayers))

	for _, layer := range origLayers {
		h, err := layer.Digest()
		if err != nil {
			return nil, err
		}
		digest := h.String()

		pl := &progressLayer{
			Layer:    layer,
			digest:   digest,
			progress: progress,
		}
		wrapped = append(wrapped, pl)
		byDigest[digest] = pl
	}

	return &progressImage{
		Image:          img,
		progress:       progress,
		layers:         wrapped,
		layersByDigest: byDigest,
	}, nil
}

func (p *progressImage) Layers() ([]v1.Layer, error) {
	return p.layers, nil
}

func (p *progressImage) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	if l, ok := p.layersByDigest[h.String()]; ok {
		return l, nil
	}
	return p.Image.LayerByDigest(h)
}

type progressStateKey struct{}

func withProgressState(ctx context.Context, p *progressState) context.Context {
	return context.WithValue(ctx, progressStateKey{}, p)
}

func getProgressState(ctx context.Context) *progressState {
	v := ctx.Value(progressStateKey{})
	if v == nil {
		return nil
	}
	p, _ := v.(*progressState)
	return p
}
