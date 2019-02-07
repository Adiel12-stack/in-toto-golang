package in_toto

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"
)

const testData = "../test/data"

// TestMain calls all Test*'s of this package (intoto) explicitly with m.Run
// This can be used for test setup and teardown, e.g. copy test data to a tmp
// test dir, change to that dir and remove the and contents in the end
func TestMain(m *testing.M) {
	testDir, err := ioutil.TempDir("", "in_toto_test_dir")
	if err != nil {
		panic("Cannot create temp test dir")
	}

	// Copy test files to temp test directory
	// NOTE: Only works for a flat directory of files
	testFiles, _ := filepath.Glob(filepath.Join(testData, "*"))
	for _, inputPath := range testFiles {
		input, err := ioutil.ReadFile(inputPath)
		if err != nil {
			panic(fmt.Sprintf("Cannot copy test files (read error: %s)", err))
		}
		outputPath := filepath.Join(testDir, filepath.Base(inputPath))
		err = ioutil.WriteFile(outputPath, input, 0644)
		if err != nil {
			panic(fmt.Sprintf("Cannot copy test files (write error: %s)", err))
		}
	}

	cwd, _ := os.Getwd()
	os.Chdir(testDir)

	// Always change back to where we were and remove the temp directory
	defer os.Chdir(cwd)
	defer os.RemoveAll(testDir)

	// Run tests
	os.Exit(m.Run())
}

func TestInTotoVerifyPass(t *testing.T) {
	// TODO: The test layout has a hardcoded expiration date. We need to
	// implement signing and create the date and sign the layout on the fly.
	layoutPath := "demo.layout.template"
	pubKeyPath := "alice.pub"
	linkDir := "."

	var layoutMb Metablock
	if err := layoutMb.Load(layoutPath); err != nil {
		t.Error(err)
	}

	var pubKey Key
	if err := pubKey.LoadPublicKey(pubKeyPath); err != nil {
		t.Error(err)
	}

	var layouKeys = map[string]Key{
		pubKey.KeyId: pubKey,
	}

	// No error should occur
	if _, err := InTotoVerify(layoutMb, layouKeys, linkDir); err != nil {
		t.Error(err)
	}
}

func TestGetSummaryLink(t *testing.T) {
	var demoLayout Metablock
	if err := demoLayout.Load("demo.layout.template"); err != nil {
		t.Error(err)
	}
	var codeLink Metablock
	if err := codeLink.Load("write-code.776a00e2.link"); err != nil {
		t.Error(err)
	}
	var packageLink Metablock
	if err := packageLink.Load("package.2f89b927.link"); err != nil {
		t.Error(err)
	}
	demoLink := make(map[string]Metablock)
	demoLink["write-code"] = codeLink
	demoLink["package"] = packageLink

	var summaryLink Metablock
	var err error
	if summaryLink, err = GetSummaryLink(demoLayout.Signed.(Layout), demoLink); err != nil {
		t.Error(err)
	}
	if summaryLink.Signed.(Link).Type != codeLink.Signed.(Link).Type {
		t.Errorf("Summary Link isn't of type Link")
	}
	if !reflect.DeepEqual(summaryLink.Signed.(Link).Name,
		codeLink.Signed.(Link).Name) {
		t.Errorf("Summary Link name doesn't match. Expected '%s', returned '%s",
			codeLink.Signed.(Link).Name, summaryLink.Signed.(Link).Name)
	}
	if !reflect.DeepEqual(summaryLink.Signed.(Link).Materials,
		codeLink.Signed.(Link).Materials) {
		t.Errorf("Summary Link materials don't match. Expected '%s', returned '%s",
			codeLink.Signed.(Link).Materials, summaryLink.Signed.(Link).Materials)
	}

	if !reflect.DeepEqual(summaryLink.Signed.(Link).Products,
		packageLink.Signed.(Link).Products) {
		t.Errorf("Summary Link products don't match. Expected '%s', returned '%s",
			packageLink.Signed.(Link).Products, summaryLink.Signed.(Link).Products)
	}
	if !reflect.DeepEqual(summaryLink.Signed.(Link).Command,
		packageLink.Signed.(Link).Command) {
		t.Errorf("Summary Link command doesn't match. Expected '%s', returned '%s",
			packageLink.Signed.(Link).Command, summaryLink.Signed.(Link).Command)
	}
	if !reflect.DeepEqual(summaryLink.Signed.(Link).ByProducts,
		packageLink.Signed.(Link).ByProducts) {
		t.Errorf("Summary Link by-products don't match. Expected '%s', returned '%s",
			packageLink.Signed.(Link).ByProducts, summaryLink.Signed.(Link).ByProducts)
	}
	if !reflect.DeepEqual(summaryLink.Signed.(Link).ByProducts["return-value"],
		packageLink.Signed.(Link).ByProducts["return-value"]) {
		t.Errorf("Summary Link return value doesn't match. Expected '%s', returned '%s",
			packageLink.Signed.(Link).ByProducts["return-value"],
			summaryLink.Signed.(Link).ByProducts["return-value"])
	}
}

func TestVerifySublayouts(t *testing.T) {
	sublayoutName := "sub_layout"
	var aliceKey Key
	if err := aliceKey.LoadPublicKey("alice.pub"); err != nil {
		t.Errorf("Unable to load Alice's public key.")
	}
	sublayoutDirectory := fmt.Sprintf(SublayoutLinkDirFormat, sublayoutName, aliceKey.KeyId)
	if err := os.Mkdir(sublayoutDirectory, 0700); err != nil {
		t.Errorf("Unable to create sublayout directory.")
	}
	writeCodePath := path.Join(sublayoutDirectory, "write-code.776a00e2.link")
	if err := os.Link("write-code.776a00e2.link", writeCodePath); err != nil {
		t.Errorf("Unable to link write-code metadata.")
	}
	packagePath := path.Join(sublayoutDirectory, "package.2f89b927.link")
	if err := os.Link("package.2f89b927.link", packagePath); err != nil {
		t.Errorf("Unable to link package metadata.")
	}

	var superLayoutMb Metablock
	if err := superLayoutMb.Load("super.layout"); err != nil {
		t.Errorf("Unable to load super layout")
	}

	stepsMetadata := make(map[string]map[string]Metablock)
	var err error
	if stepsMetadata, err = LoadLinksForLayout(superLayoutMb.Signed.(Layout), "."); err != nil {
		t.Errorf("Unable to load link metadata for super layout.")
	}

	stepsMetadataVerified := make(map[string]map[string]Metablock)
	if stepsMetadataVerified, err = VerifyLinkSignatureThesholds(superLayoutMb.Signed.(Layout), stepsMetadata); err != nil {
		t.Errorf("Unable to verify link threshold values.")
	}

	if _, err := VerifySublayouts(superLayoutMb.Signed.(Layout), stepsMetadataVerified, "."); err != nil {
		t.Errorf("Unable to verify sublayouts.")
	}

	if err := os.RemoveAll(sublayoutDirectory); err != nil {
		t.Errorf("Failed to remove sublayout link directory.")
	}

}
