package image

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/shuveb/containers-the-hard-way/utils"
	"github.com/shuveb/containers-the-hard-way/workdirs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
)

type Manifest struct {
	Config string
	RepoTags []string
	Layers []string
}

type ConfigDetails struct {
	Env []string	`json:"Env"`
	Cmd []string	`json:"Cmd"`
}

type Config struct {
	Config ConfigDetails `json:"config"`
}

/*
This is the format of our imageDB file where we store the
list of images we have on the system.
{
	"ubuntu" : {
					"18.04": "[image-hash]",
					"18.10": "[image-hash]",
					"19.04": "[image-hash]",
					"19.10": "[image-hash]",
				},
	"centos" : {
					"6.0": "[image-hash]",
					"6.1": "[image-hash]",
					"6.2": "[image-hash]",
					"7.0": "[image-hash]",
				}
}
*/

type imageEntries map[string]string
type imagesDB map[string]imageEntries

type Accessor struct {}

func GetAccessor() Accessor {
	return Accessor{}
}

func (i Accessor) GetImageAndTagByHash(imageShaHash string) (string, string) {
	idb := imagesDB{}
	i.parseImagesMetadata(&idb)
	for image,versions := range idb {
		for version, hash := range versions {
			if hash == imageShaHash {
				return image, version
			}
		}
	}
	return "", ""
}

func (i Accessor) GetBasePathForImage(imageShaHex string) string {
	return path.Join(workdirs.ImagesPath(), imageShaHex)
}

func (i Accessor) GetManifestPathForImage(imageShaHex string) string {
	return path.Join(i.GetBasePathForImage(imageShaHex), "manifest.json")
}

func (i Accessor) GetConfigPathForImage(imageShaHex string) string {
	return path.Join(i.GetBasePathForImage(imageShaHex), imageShaHex + ".json")
}

func (i Accessor) deleteTempImageFiles(imageShaHash string) {
	tmpPath := path.Join(workdirs.TempPath(), imageShaHash)
	utils.MustWithMsg(os.RemoveAll(tmpPath),
		"Unable to remove temporary image files")
}

func (i Accessor) imageExistsByHash(imageShaHex string) (string, string) {
	idb := imagesDB{}
	i.parseImagesMetadata(&idb)
	for imgName, avlImages := range idb {
		for imgTag, imgHash := range avlImages {
			if imgHash == imageShaHex {
				return imgName, imgTag
			}
		}
	}
	return "", ""
}

func (i Accessor) imageExistByTag(imgName string, tagName string) (bool, string) {
	idb := imagesDB{}
	i.parseImagesMetadata(&idb)
	for k, v := range idb {
		if k == imgName {
			for k, v := range v {
				if k == tagName {
					return true, v
				}
			}
		}
	}
	return false, ""
}

func (i Accessor) downloadImage(img v1.Image, imageShaHex string, src string) {
	imagePath := path.Join(workdirs.TempPath(), imageShaHex)
	utils.Must(os.Mkdir(imagePath, 0755))
	tarPath := path.Join(imagePath, "package.tar")
	/* Save the image as a tar file */
	if err := crane.SaveLegacy(img, src, tarPath); err != nil {
		log.Fatalf("saving tarball %s: %v", tarPath, err)
	}
	log.Printf("Successfully downloaded %s\n", src)
}

func (i Accessor) unTarFile(imageShaHex string) {
	pathDir := path.Join(workdirs.TempPath(), imageShaHex)
	tarPath := path.Join(pathDir, "package.tar")
	if err := utils.UnTar(tarPath, pathDir); err != nil {
		log.Fatalf("Error untaring file: %v\n", err)
	}
}

func (i Accessor) processLayerTarballs(imageShaHex string, fullImageHex string) {
	tmpPathDir := path.Join(workdirs.TempPath(), imageShaHex)
	pathManifest := path.Join(tmpPathDir, "manifest.json")
	pathConfig := path.Join(tmpPathDir, fullImageHex + ".json")

	mani := i.ParseManifest(pathManifest)
	imagesDir := path.Join(workdirs.ImagesPath(), imageShaHex)
	_ = os.Mkdir(imagesDir, 0755)
	/* untar the layer files. These become the basis of our container root fs */
	for _, layer := range mani.Layers {
		imageLayerDir := path.Join(imagesDir, layer[:12], "fs")
		log.Printf("Uncompressing layer to: %s \n", imageLayerDir)
		_ = os.MkdirAll(imageLayerDir, 0755)
		srcLayer := path.Join(tmpPathDir, layer)
		if err:= utils.UnTar(srcLayer, imageLayerDir); err != nil {
			log.Fatalf("Unable to untar layer file: %s: %v\n", srcLayer, err)
		}
	}
	/* Copy the Manifest file for reference later */
	err := utils.CopyFile(pathManifest, i.GetManifestPathForImage(imageShaHex))
	if err != nil {
		log.Printf("[warning]")
	}
	utils.CopyFile(pathConfig, i.GetConfigPathForImage(imageShaHex))
}

func (i Accessor) ParseContainerConfig(imageShaHex string) Config {
	imagesConfigPath := i.GetConfigPathForImage(imageShaHex)
	data, err := ioutil.ReadFile(imagesConfigPath)
	if err != nil {
		log.Fatalf("Could not read image config file")
	}
	imgConfig := Config{}
	if err := json.Unmarshal(data, &imgConfig); err != nil {
		log.Fatalf("Unable to parse image config data!")
	}
	return imgConfig
}

func (i Accessor) parseImagesMetadata(idb *imagesDB)  {
	imagesDBPath := path.Join(workdirs.ImagesPath(), "images.json")
	if _, err := os.Stat(imagesDBPath); os.IsNotExist(err) {
		/* If it doesn't exist create an empty DB */
		ioutil.WriteFile(imagesDBPath, []byte("{}"), 0644)
	}
	data, err := ioutil.ReadFile(imagesDBPath)
	if err != nil {
		log.Fatalf("Could not read images DB: %v\n", err)
	}
	if err := json.Unmarshal(data, idb); err != nil {
		log.Fatalf("Unable to parse images DB: %v\n", err)
	}
}

func (i Accessor) marshalImageMetadata(idb imagesDB) {
	fileBytes, err := json.Marshal(idb)
	if err != nil {
		log.Fatalf("Unable to marshall images data: %v\n", err)
	}
	imagesDBPath := path.Join(workdirs.ImagesPath(), "images.json")
	if err := ioutil.WriteFile(imagesDBPath, fileBytes, 0644); err != nil {
		log.Fatalf("Unable to save images DB: %v\n", err)
	}
}

func (i Accessor) storeImageMetadata(image string, tag string, imageShaHex string) {
	idb := imagesDB{}
	ientry := imageEntries{}
	i.parseImagesMetadata(&idb)
	if idb[image] != nil {
		ientry = idb[image]
	}
	ientry[tag] = imageShaHex
	idb[image] = ientry

	i.marshalImageMetadata(idb)
}

func (i Accessor) removeImageMetadata(imageShaHex string) {
	idb := imagesDB{}
	ientries := imageEntries{}
	i.parseImagesMetadata(&idb)
	imgName, _ := i.imageExistsByHash(imageShaHex)
	if len(imgName) == 0 {
		log.Fatalf("Could not get image details")
	}
	ientries = idb[imgName]
	for tag, hash := range ientries {
		if hash == imageShaHex {
			delete(ientries, tag)
		}
	}
	if len(ientries) == 0 {
		delete(idb, imgName)
	} else {
		idb[imgName] = ientries
	}
	i.marshalImageMetadata(idb)
}

func (i Accessor) DeleteImageByHash(imageShaHex string) {
	utils.MustWithMsg(os.RemoveAll(path.Join(workdirs.ImagesPath(), imageShaHex)),
		"Unable to remove image directory")
	i.removeImageMetadata(imageShaHex)
}

func (i Accessor) PrintAvailableImages() {
	idb := imagesDB{}
	i.parseImagesMetadata(&idb)
	fmt.Printf("IMAGE\t             TAG\t   ID\n")
	for image, details := range idb {
		fmt.Println(image)
		for tag, hash := range details {
			fmt.Printf("\t%16s %s\n", tag, hash)
		}
	}
}

func (i Accessor) GetImageNameAndTag(src string) (string, string) {
	s := strings.Split(src, ":")
	var img, tag string
	if len(s) > 1 {
		img, tag = s[0], s[1]
	} else {
		img = s[0]
		tag = "latest"
	}
	return img, tag
}

func (i Accessor) DownloadImageIfRequired(src string) string {
	imgName, tagName := i.GetImageNameAndTag(src)
	if downloadRequired, imageShaHex := i.imageExistByTag(imgName, tagName); !downloadRequired {
		/* Setup the image we want to pull */
		log.Printf("Downloading metadata for %s:%s, please wait...", imgName, tagName)
		img, err := crane.Pull(strings.Join([]string{imgName, tagName}, ":"))
		if err != nil {
			log.Fatal(err)
		}

		manifest, _ := img.Manifest()
		imageShaHex = manifest.Config.Digest.Hex[:12]
		log.Printf("imageHash: %v\n", imageShaHex)
		log.Println("Checking if image exists under another name...")
		/* Identify cases where ubuntu:latest could be the same as ubuntu:20.04*/
		altImgName, altImgTag := i.imageExistsByHash(imageShaHex)
		if len(altImgName) > 0 && len(altImgTag) > 0 {
			log.Printf("The image you requested %s:%s is the same as %s:%s\n",
				imgName, tagName, altImgName, altImgTag)
			i.storeImageMetadata(imgName, tagName, imageShaHex)
			return imageShaHex
		} else {
			log.Println("Image doesn't exist. Downloading...")
			i.downloadImage(img, imageShaHex, src)
			i.unTarFile(imageShaHex)
			i.processLayerTarballs(imageShaHex, manifest.Config.Digest.Hex)
			i.storeImageMetadata(imgName, tagName, imageShaHex)
			i.deleteTempImageFiles(imageShaHex)
			return imageShaHex
		}
	} else {
		log.Println("Image already exists. Not downloading.")
		return imageShaHex
	}
}

func (i Accessor) ParseManifest(manifestPath string) *Manifest {
	m := make([]*Manifest, 0)
	data, err := ioutil.ReadFile(manifestPath)
	utils.Must(err)
	utils.Must(json.Unmarshal(data, &m))
	if len(m) == 0 || len(m) > 1 {
		log.Fatalf("ParseManifest failed, manifest count = %d", len(m))
	}
	return m[0]
}