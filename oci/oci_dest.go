package oci

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
)

type ociManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Config        descriptor        `json:"config"`
	Layers        []descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations"`
}

type descriptor struct {
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
}

type ociImageDestination struct {
	ref ociReference
}

// newImageDestination returns an ImageDestination for writing to an existing directory.
func newImageDestination(ref ociReference) types.ImageDestination {
	return &ociImageDestination{ref: ref}
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *ociImageDestination) Reference() types.ImageReference {
	return d.ref
}

func createManifest(m []byte) ([]byte, string, error) {
	om := ociManifest{}
	mt := manifest.GuessMIMEType(m)
	switch mt {
	case manifest.DockerV2Schema1MIMEType:
		// There a simple reason about not yet implementing this.
		// OCI image-spec assure about backward compatibility with docker v2s2 but not v2s1
		// generating a v2s2 is a migration docker does when upgrading to 1.10.3
		// and I don't think we should bother about this now (I don't want to have migration code here in skopeo)
		return nil, "", fmt.Errorf("can't create OCI manifest from Docker V2 schema 1 manifest")
	case manifest.DockerV2Schema2MIMEType:
		if err := json.Unmarshal(m, &om); err != nil {
			return nil, "", err
		}
		om.MediaType = manifest.OCIV1ImageManifestMIMEType
		for i := range om.Layers {
			om.Layers[i].MediaType = manifest.OCIV1ImageSerializationMIMEType
		}
		om.Config.MediaType = manifest.OCIV1ImageSerializationConfigMIMEType
		b, err := json.Marshal(om)
		if err != nil {
			return nil, "", err
		}
		return b, om.MediaType, nil
	case manifest.DockerV2ListMIMEType:
		return nil, "", fmt.Errorf("can't create OCI manifest from Docker V2 schema 2 manifest list")
	case manifest.OCIV1ImageManifestListMIMEType:
		return nil, "", fmt.Errorf("can't create OCI manifest from OCI manifest list")
	case manifest.OCIV1ImageManifestMIMEType:
		return m, om.MediaType, nil
	}
	return nil, "", fmt.Errorf("Unrecognized manifest media type")
}

func (d *ociImageDestination) PutManifest(m []byte) error {
	if err := d.ensureParentDirectoryExists("refs"); err != nil {
		return err
	}
	// TODO(mitr, runcom): this breaks signatures entirely since at this point we're creating a new manifest
	// and signatures don't apply anymore. Will fix.
	ociMan, mt, err := createManifest(m)
	if err != nil {
		return err
	}
	digest, err := manifest.Digest(ociMan)
	if err != nil {
		return err
	}
	desc := descriptor{}
	desc.Digest = digest
	// TODO(runcom): beaware and add support for OCI manifest list
	desc.MediaType = mt
	desc.Size = int64(len(ociMan))
	data, err := json.Marshal(desc)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(blobPath(d.ref.dir, digest), ociMan, 0644); err != nil {
		return err
	}
	// TODO(runcom): ugly here?
	if err := ioutil.WriteFile(ociLayoutPath(d.ref.dir), []byte(`{"imageLayoutVersion": "1.0.0"}`), 0644); err != nil {
		return err
	}
	return ioutil.WriteFile(descriptorPath(d.ref.dir, d.ref.tag), data, 0644)
}

func (d *ociImageDestination) PutBlob(digest string, stream io.Reader) error {
	if err := d.ensureParentDirectoryExists("blobs"); err != nil {
		return err
	}
	blob, err := os.Create(blobPath(d.ref.dir, digest))
	if err != nil {
		return err
	}
	defer blob.Close()
	if _, err := io.Copy(blob, stream); err != nil {
		return err
	}
	if err := blob.Sync(); err != nil {
		return err
	}
	return nil
}

func (d *ociImageDestination) ensureParentDirectoryExists(parent string) error {
	path := filepath.Join(d.ref.dir, parent)
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (d *ociImageDestination) SupportedManifestMIMETypes() []string {
	return []string{
		manifest.OCIV1ImageManifestMIMEType,
		manifest.DockerV2Schema2MIMEType,
	}
}

func (d *ociImageDestination) PutSignatures(signatures [][]byte) error {
	if len(signatures) != 0 {
		return fmt.Errorf("Pushing signatures for OCI images is not supported")
	}
	return nil
}

// ociLayoutPathPath returns a path for the oci-layout within a directory using OCI conventions.
func ociLayoutPath(dir string) string {
	return filepath.Join(dir, "oci-layout")
}

// blobPath returns a path for a blob within a directory using OCI image-layout conventions.
func blobPath(dir string, digest string) string {
	return filepath.Join(dir, "blobs", strings.Replace(digest, ":", "-", -1))
}

// descriptorPath returns a path for the manifest within a directory using OCI conventions.
func descriptorPath(dir string, digest string) string {
	return filepath.Join(dir, "refs", digest)
}
