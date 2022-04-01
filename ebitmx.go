package ebitmx

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

// EbitenMap is the transformed representation of a TMX map in the simplest
// way possible for Ebiten to understand and render
type EbitenMap struct {
	TileWidth    int
	TileHeight   int
	MapHeight    int
	MapWidth     int
	Layers       [][]uint32
	TilesetInfos []TilesetInfo
}

// GetEbitenMap returns a map that Ebiten can understand
// based on a TMX file. Note that some data might be lost, as Ebiten
// does not require too much information to render a map
func GetEbitenMap(path string) (*EbitenMap, error) {
	return GetEbitenMapFromFS(os.DirFS("."), path)
}

// GetEbitenMapFromFS allows you to pass in the file system used to find the desired file
// This is useful for Go's v1.16 embed package which makes it simple to embed assets into
// your binary and accessible via the embed.FS which is compatible with the fs.FS interface
func GetEbitenMapFromFS(fileSystem fs.FS, path string) (*EbitenMap, error) {
	tmxFile, err := fileSystem.Open(path)

	if err != nil {
		return nil, fmt.Errorf("error opening TMX file %s: %v", path, err)
	}

	defer tmxFile.Close()

	bytes, err := ioutil.ReadAll(tmxFile)
	if err != nil {
		return nil, fmt.Errorf("error reading TMX file %s: %v", path, err)
	}

	tmxMap, err := ParseTMX(bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing TMX file %s: %v", path, err)
	}

	return transformMapToEbitenMap(tmxMap)
}

func transformMapToEbitenMap(tmx *Map) (*EbitenMap, error) {
	ebitenMap := &EbitenMap{
		TileWidth:  tmx.TilHeight,
		TileHeight: tmx.TileWidth,
		MapHeight:  tmx.Height,
		MapWidth:   tmx.Width,
	}

	var ebitenLayers [][]uint32
	for _, layer := range tmx.Layers {
		var innerLayer []uint32
		var err error
		var base64Bytes []byte
		if layer.Data.Encoding == "csv" {
			for _, s := range strings.Split(string(layer.Data.Raw), ",") {
				s = strings.TrimSpace(s)
				coord, err := strconv.Atoi(s)

				if err != nil {
					return nil, fmt.Errorf("error parsing layer [%s] data, %v is not a number", layer.Name, s)
				}
				innerLayer = append(innerLayer, uint32(coord))
			}
		} else if layer.Data.Encoding == "base64" {
			r := strings.TrimSpace(string(layer.Data.Raw))
			base64Bytes, err = base64.StdEncoding.DecodeString(r)
			if err != nil {
				return nil, err
			}

			var uncompress []byte
			br := bytes.NewReader(base64Bytes)
			switch layer.Data.Compression {
			case "zlib":
				fmt.Println("zlib")
				zlibReader, err := zlib.NewReader(br)
				if err != nil {
					return nil, err
				}
				uncompress, err = extract(zlibReader)
				break
			case "gzip":
				fmt.Println("gzip")
				gzipReader, err := gzip.NewReader(br)
				if err != nil {
					return nil, err
				}
				uncompress, err = extract(gzipReader)
			case "":
				uncompress = base64Bytes
				break
			default:
				return nil, fmt.Errorf("unknown compress format")

			}

			buf := make([]byte, 4)
			br = bytes.NewReader(uncompress)
			for {
				n, err := br.Read(buf)
				if err == io.EOF {
					// there is no more data to read
					break
				}
				if err != nil {
					fmt.Println(err)
					continue
				}
				if n > 0 {
					data := binary.LittleEndian.Uint32(buf)
					innerLayer = append(innerLayer, data)
				}
			}
		}

		ebitenLayers = append(ebitenLayers, innerLayer)
	}

	ebitenMap.Layers = ebitenLayers
	ebitenMap.TilesetInfos = make([]TilesetInfo, len(tmx.Tileset))
	for i, ts := range tmx.Tileset {
		ebitenMap.TilesetInfos[i] = ts
	}
	return ebitenMap, nil
}

func extract(reader io.Reader) ([]byte, error) {
	base64Bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return base64Bytes, nil
}
