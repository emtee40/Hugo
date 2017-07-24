// Copyright 2017-present The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gohugoio/hugo/helpers"
)

type imageCache struct {
	absPublishDir string
	absCacheDir   string
	pathSpec      *helpers.PathSpec
	mu            sync.RWMutex
	store         map[string]*Image
}

func (c *imageCache) isInCache(key string) bool {
	c.mu.RLock()
	_, found := c.store[key]
	c.mu.RUnlock()
	return found
}

func (c *imageCache) deleteByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, _ := range c.store {
		if strings.HasPrefix(k, prefix) {
			delete(c.store, k)
		}
	}
}

func (c *imageCache) getOrCreate(
	spec *Spec, key string, create func(resourceCacheFilename string) (*Image, error)) (*Image, error) {
	// First check the in-memory store, then the disk.
	c.mu.RLock()
	img, found := c.store[key]
	c.mu.RUnlock()

	if found {
		return img, nil
	}

	// Now look in the file cache.
	cacheFilename := filepath.Join(c.absCacheDir, key)

	// The definition of this counter is not that we have processed that amount
	// (e.g. resized etc.), it can be fetched from file cache,
	//  but the count of processed image variations for this site.
	c.pathSpec.ProcessingStats.Incr(&c.pathSpec.ProcessingStats.ProcessedImages)

	r, err := spec.NewResourceFromFilename(nil, c.absPublishDir, cacheFilename, key)
	notFound := err != nil && os.IsNotExist(err)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if notFound {
		img, err = create(cacheFilename)
		if err != nil {
			return nil, err
		}
	} else {
		img = r.(*Image)
	}

	c.mu.Lock()
	if img2, found := c.store[key]; found {
		c.mu.Unlock()
		return img2, nil
	}

	c.store[key] = img

	c.mu.Unlock()

	if notFound {
		// File already written to destination
		return img, nil
	}

	return img, img.copyToDestination(cacheFilename)

}

func newImageCache(ps *helpers.PathSpec, absCacheDir, absPublishDir string) *imageCache {
	return &imageCache{pathSpec: ps, store: make(map[string]*Image), absCacheDir: absCacheDir, absPublishDir: absPublishDir}
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s took %s\n", name, elapsed)
}
