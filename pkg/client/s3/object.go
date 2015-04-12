// Original license //
// ---------------- //

/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// All other modifications and improvements //
// ---------------------------------------- //

/*
 * Modern Copy, (C) 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package s3

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"encoding/base64"
	"encoding/hex"
	"net/http"

	"github.com/minio-io/mc/pkg/client"
	"github.com/minio-io/minio/pkg/iodine"
	"sync"
)

/// Object API operations

// Put - upload new object to bucket
func (c *s3Client) Put(bucket, key, md5HexString string, size int64) (io.WriteCloser, error) {
	r, w := io.Pipe()
	blockingWriter := NewBlockingWriteCloser(w)
	go func() {
		req := newReq(c.keyURL(bucket, key), c.UserAgent, r)
		req.Method = "PUT"
		req.ContentLength = size

		// set Content-MD5 only if md5 is provided
		if strings.TrimSpace(md5HexString) != "" {
			md5, err := hex.DecodeString(md5HexString)
			if err != nil {
				err := iodine.New(err, nil)
				r.CloseWithError(err)
				blockingWriter.Release(err)
				return
			}
			req.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(md5))
		}
		c.signRequest(req, c.Host)

		client := http.Client{}
		res, err := client.Do(req)

		if err != nil {
			err := iodine.New(err, nil)
			r.CloseWithError(err)
			blockingWriter.Release(err)
			return
		}

		if res.StatusCode != http.StatusOK {
			err := iodine.New(err, nil)
			r.CloseWithError(err)
			blockingWriter.Release(err)
			return
		}
		blockingWriter.Release(nil)
		r.Close()
	}()
	return blockingWriter, nil
}

type blockingWriteCloser struct {
	w       io.WriteCloser
	release *sync.WaitGroup
	err     error
}

func (b *blockingWriteCloser) Write(p []byte) (int, error) {
	n, err := b.w.Write(p)
	err = iodine.New(err, nil)
	return n, err
}

func (b *blockingWriteCloser) Close() error {
	err := b.w.Close()
	if err != nil {
		b.err = err
	}
	b.release.Wait()
	return b.err
}

func (b *blockingWriteCloser) Release(err error) {
	b.release.Done()
	if err != nil {
		b.err = err
	}
}

func NewBlockingWriteCloser(w io.WriteCloser) *blockingWriteCloser {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &blockingWriteCloser{w: w, release: wg}
}

// Stat - returns 0, "", os.ErrNotExist if not on S3
func (c *s3Client) StatObject(bucket, key string) (size int64, date time.Time, reterr error) {
	if bucket == "" || key == "" {
		return 0, date, iodine.New(client.InvalidArgument{}, nil)
	}
	req := newReq(c.keyURL(bucket, key), c.UserAgent, nil)
	req.Method = "HEAD"
	c.signRequest(req, c.Host)
	res, err := c.Transport.RoundTrip(req)
	if err != nil {
		return 0, date, iodine.New(err, nil)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusNotFound:
		return 0, date, iodine.New(client.ObjectNotFound{Bucket: bucket, Object: key}, nil)
	case http.StatusOK:
		size, err = strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
		if err != nil {
			return 0, date, iodine.New(err, nil)
		}
		if dateStr := res.Header.Get("Last-Modified"); dateStr != "" {
			// AWS S3 uses RFC1123 standard for Date in HTTP header, unlike XML content
			date, err := time.Parse(time.RFC1123, dateStr)
			if err != nil {
				return 0, date, iodine.New(err, nil)
			}
			return size, date, nil
		}
	default:
		return 0, date, iodine.New(NewError(res), nil)
	}
	return
}

// Get - download a requested object from a given bucket
func (c *s3Client) Get(bucket, key string) (body io.ReadCloser, size int64, md5 string, err error) {
	req := newReq(c.keyURL(bucket, key), c.UserAgent, nil)
	c.signRequest(req, c.Host)
	res, err := c.Transport.RoundTrip(req)
	if err != nil {
		return nil, 0, "", iodine.New(err, nil)
	}

	if res.StatusCode != http.StatusOK {
		return nil, 0, "", iodine.New(NewError(res), nil)
	}
	md5sum := strings.Trim(res.Header.Get("ETag"), "\"") // trim off the erroneous double quotes
	return res.Body, res.ContentLength, md5sum, nil
}

// GetPartial fetches part of the s3 key object in bucket.
// If length is negative, the rest of the object is returned.
func (c *s3Client) GetPartial(bucket, key string, offset, length int64) (body io.ReadCloser, size int64, md5 string, err error) {
	if offset < 0 {
		return nil, 0, "", iodine.New(client.InvalidRange{Offset: offset}, nil)
	}
	req := newReq(c.keyURL(bucket, key), c.UserAgent, nil)
	if length >= 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	} else {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	c.signRequest(req, c.Host)

	res, err := c.Transport.RoundTrip(req)
	if err != nil {
		return nil, 0, "", iodine.New(err, nil)
	}

	switch res.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		return res.Body, res.ContentLength, res.Header.Get("ETag"), nil
	default:
		return nil, 0, "", iodine.New(NewError(res), nil)
	}
}
