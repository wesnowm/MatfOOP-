// +build !containers_image_storage_stub

package storage

import (
	"context"
	"strings"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// A storageReference holds an arbitrary name and/or an ID, which is a 32-byte
// value hex-encoded into a 64-character string, and a reference to a Store
// where an image is, or would be, kept.
type storageReference struct {
	transport            storageTransport
	completeReference    reference.Named // may include a tag and/or a digest
	reference            string
	id                   string
	name                 reference.Named // May include a tag, not a digest
	breakDockerReference bool            // Possibly set by newImageDestination.  FIXME: Figure out another way.
}

func newReference(transport storageTransport, completeReference reference.Named, reference, id string, name reference.Named) *storageReference {
	// We take a copy of the transport, which contains a pointer to the
	// store that it used for resolving this reference, so that the
	// transport that we'll return from Transport() won't be affected by
	// further calls to the original transport's SetStore() method.
	return &storageReference{
		transport:         transport,
		completeReference: completeReference,
		reference:         reference,
		id:                id,
		name:              name,
	}
}

// imageMatchesRepo returns true iff image.Names contains an element with the same repo as ref
func imageMatchesRepo(image *storage.Image, ref reference.Named) bool {
	repo := ref.Name()
	for _, name := range image.Names {
		if named, err := reference.ParseNormalizedNamed(name); err == nil {
			if named.Name() == repo {
				return true
			}
		}
	}
	return false
}

// Resolve the reference's name to an image ID in the store, if there's already
// one present with the same name or ID, and return the image.
func (s *storageReference) resolveImage() (*storage.Image, error) {
	if s.id == "" {
		// Look for an image that has the expanded reference name as an explicit Name value.
		image, err := s.transport.store.Image(s.reference)
		if image != nil && err == nil {
			s.id = image.ID
		}
	}
	if s.id == "" && s.completeReference != nil {
		if digested, ok := s.completeReference.(reference.Digested); ok {
			// Look for an image with the specified digest that has the same name,
			// though possibly with a different tag or digest, as a Name value, so
			// that the canonical reference can be implicitly resolved to the image.
			images, err := s.transport.store.ImagesByDigest(digested.Digest())
			if images != nil && err == nil {
				for _, image := range images {
					if imageMatchesRepo(image, s.completeReference) {
						s.id = image.ID
						break
					}
				}
			}
		}
	}
	if s.id == "" {
		logrus.Debugf("reference %q does not resolve to an image ID", s.StringWithinTransport())
		return nil, errors.Wrapf(ErrNoSuchImage, "reference %q does not resolve to an image ID", s.StringWithinTransport())
	}
	img, err := s.transport.store.Image(s.id)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading image %q", s.id)
	}
	if s.completeReference != nil {
		if !imageMatchesRepo(img, s.completeReference) {
			logrus.Errorf("no image matching reference %q found", s.StringWithinTransport())
			return nil, ErrNoSuchImage
		}
	}
	return img, nil
}

// Return a Transport object that defaults to using the same store that we used
// to build this reference object.
func (s storageReference) Transport() types.ImageTransport {
	return &storageTransport{
		store:         s.transport.store,
		defaultUIDMap: s.transport.defaultUIDMap,
		defaultGIDMap: s.transport.defaultGIDMap,
	}
}

// Return a name with a tag or digest, if we have either, else return it bare.
func (s storageReference) DockerReference() reference.Named {
	if s.breakDockerReference {
		return nil
	}
	return s.completeReference
}

// Return a name with a tag, prefixed with the graph root and driver name, to
// disambiguate between images which may be present in multiple stores and
// share only their names.
func (s storageReference) StringWithinTransport() string {
	optionsList := ""
	options := s.transport.store.GraphOptions()
	if len(options) > 0 {
		optionsList = ":" + strings.Join(options, ",")
	}
	storeSpec := "[" + s.transport.store.GraphDriverName() + "@" + s.transport.store.GraphRoot() + "+" + s.transport.store.RunRoot() + optionsList + "]"
	if s.reference == "" {
		return storeSpec + "@" + s.id
	}
	if s.id == "" {
		return storeSpec + s.reference
	}
	return storeSpec + s.reference + "@" + s.id
}

func (s storageReference) PolicyConfigurationIdentity() string {
	storeSpec := "[" + s.transport.store.GraphDriverName() + "@" + s.transport.store.GraphRoot() + "]"
	if s.completeReference == nil {
		return storeSpec + "@" + s.id
	}
	if s.id == "" {
		return storeSpec + s.reference
	}
	return storeSpec + s.reference + "@" + s.id
}

// Also accept policy that's tied to the combination of the graph root and
// driver name, to apply to all images stored in the Store, and to just the
// graph root, in case we're using multiple drivers in the same directory for
// some reason.
func (s storageReference) PolicyConfigurationNamespaces() []string {
	storeSpec := "[" + s.transport.store.GraphDriverName() + "@" + s.transport.store.GraphRoot() + "]"
	driverlessStoreSpec := "[" + s.transport.store.GraphRoot() + "]"
	namespaces := []string{}
	if s.completeReference != nil {
		if s.id != "" {
			// The reference without the ID is also a valid namespace.
			namespaces = append(namespaces, storeSpec+s.reference)
		}
		components := strings.Split(s.completeReference.Name(), "/")
		for len(components) > 0 {
			namespaces = append(namespaces, storeSpec+strings.Join(components, "/"))
			components = components[:len(components)-1]
		}
	}
	namespaces = append(namespaces, storeSpec)
	namespaces = append(namespaces, driverlessStoreSpec)
	return namespaces
}

// NewImage returns a types.ImageCloser for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned ImageCloser.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
// WARNING: This may not do the right thing for a manifest list, see image.FromSource for details.
func (s storageReference) NewImage(ctx context.Context, sys *types.SystemContext) (types.ImageCloser, error) {
	return newImage(ctx, sys, s)
}

func (s storageReference) DeleteImage(ctx context.Context, sys *types.SystemContext) error {
	img, err := s.resolveImage()
	if err != nil {
		return err
	}
	layers, err := s.transport.store.DeleteImage(img.ID, true)
	if err == nil {
		logrus.Debugf("deleted image %q", img.ID)
		for _, layer := range layers {
			logrus.Debugf("deleted layer %q", layer)
		}
	}
	return err
}

func (s storageReference) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	return newImageSource(s)
}

func (s storageReference) NewImageDestination(ctx context.Context, sys *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(s)
}
