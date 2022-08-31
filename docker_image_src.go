package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/projectatomic/skopeo/reference"
	"github.com/projectatomic/skopeo/types"
)

type errFetchManifest struct {
	statusCode int
	body       []byte
}

func (e errFetchManifest) Error() string {
	return fmt.Sprintf("error fetching manifest: status code: %d, body: %s", e.statusCode, string(e.body))
}

type dockerImageSource struct {
	ref reference.Named
	tag string
	c   *dockerClient
}

// newDockerImageSource is the same as NewDockerImageSource, only it returns the more specific *dockerImageSource type.
func newDockerImageSource(img, certPath string, tlsVerify bool) (*dockerImageSource, error) {
	ref, tag, err := parseDockerImageName(img)
	if err != nil {
		return nil, err
	}
	c, err := newDockerClient(ref.Hostname(), certPath, tlsVerify)
	if err != nil {
		return nil, err
	}
	return &dockerImageSource{
		ref: ref,
		tag: tag,
		c:   c,
	}, nil
}

// NewDockerImageSource creates a new ImageSource for the specified image and connection specification.
func NewDockerImageSource(img, certPath string, tlsVerify bool) (types.ImageSource, error) {
	return newDockerImageSource(img, certPath, tlsVerify)
}

func (s *dockerImageSource) GetManifest() (manifest []byte, unverifiedCanonicalDigest string, err error) {
	url := fmt.Sprintf(manifestURL, s.ref.RemoteName(), s.tag)
	// TODO(runcom) set manifest version header! schema1 for now - then schema2 etc etc and v1
	// TODO(runcom) NO, switch on the resulter manifest like Docker is doing
	res, err := s.c.makeRequest("GET", url, nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	manblob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode != http.StatusOK {
		return nil, "", errFetchManifest{res.StatusCode, manblob}
	}
	return manblob, res.Header.Get("Docker-Content-Digest"), nil
}

func (s *dockerImageSource) GetLayer(digest string) (io.ReadCloser, error) {
	url := fmt.Sprintf(blobsURL, s.ref.RemoteName(), digest)
	logrus.Infof("Downloading %s", url)
	res, err := s.c.makeRequest("GET", url, nil, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		// print url also
		return nil, fmt.Errorf("Invalid status code returned when fetching blob %d", res.StatusCode)
	}
	return res.Body, nil
}

func (s *dockerImageSource) GetSignatures() ([][]byte, error) {
	return [][]byte{}, nil
}
