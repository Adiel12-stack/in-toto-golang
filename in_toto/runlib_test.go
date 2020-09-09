package in_toto

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRecordArtifact(t *testing.T) {
	// Test successfully record one artifact
	result, err := RecordArtifact("foo.tar.gz")
	expected := map[string]interface{}{
		"sha256": "52947cb78b91ad01fe81cd6aef42d1f6817e92b9e6936c1e5aabb7c98514f355",
	}
	if !reflect.DeepEqual(result, expected) || err != nil {
		t.Errorf("RecordArtifact returned '(%s, %s)', expected '(%s, nil)'",
			result, err, expected)
	}

	// Test error by recording nonexistent artifact
	result, err = RecordArtifact("file-does-not-exist")
	if !os.IsNotExist(err) {
		t.Errorf("RecordArtifact returned '(%s, %s)', expected '(nil, %s)'",
			result, err, os.ErrNotExist)
	}
}

// TestSymlinkToFile checks if we can follow symlinks to a file
// Note: Symlink files are invisible for InToto right now.
// Therefore if we have a symlink like: foo.tar.gz.sym -> foo.tar.gz
// We will only calculate the hash for for.tar.gz
// The symlink will not be added to the list right now, nor will we calculate a checksum for it.
func TestSymlinkToFile(t *testing.T) {
	if err := os.Symlink("foo.tar.gz", "foo.tar.gz.sym"); err != nil {
		t.Errorf("Could not create a symlink: %s", err)
	}

	expected := map[string]interface{}{
		"foo.tar.gz": map[string]interface{}{
			"sha256": "52947cb78b91ad01fe81cd6aef42d1f6817e92b9e6936c1e5aabb7c98514f355",
		},
	}
	result, err := RecordArtifacts([]string{"foo.tar.gz.sym"})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("RecordArtifacts returned '(%s, %s)', expected '(%s, nil)'",
			result, err, expected)
	}

	if err := os.Remove("foo.tar.gz.sym"); err != nil {
		t.Errorf("Could not remove foo.tar.gz.sym: %s", err)
	}
}

// TestIndirectSymlinkCycles() tests for indirect symlink cycles in the form:
// symTestA/linkToB -> symTestB and symTestB/linkToA -> symTestA
func TestIndirectSymlinkCycles(t *testing.T) {
	if err := os.Mkdir("symTestA", 0700); err != nil {
		t.Errorf("Could not create tmpdir: %s", err)
	}
	if err := os.Mkdir("symTestB", 0700); err != nil {
		t.Errorf("Could not create tmpdir: %s", err)
	}

	// we need to get the current working directory here, otherwise
	// os.Symlink() will create a wrong symlink
	dir, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}

	linkB := filepath.FromSlash("symTestA/linkToB.sym")
	linkA := filepath.FromSlash("symTestB/linkToA.sym")

	if err := os.Symlink(dir+"/symTestA", linkA); err != nil {
		t.Errorf("Could not create a symlink: %s", err)
	}
	if err := os.Symlink(dir+"/symTestB", linkB); err != nil {
		t.Errorf("Could not create a symlink: %s", err)
	}

	// provoke "symlink cycle detected" error
	_, err = RecordArtifacts([]string{"symTestA/linkToB.sym", "symTestB/linkToA.sym", "foo.tar.gz"})
	if !errors.Is(err, ErrSymCycle) {
		t.Errorf("We expected: %s, we got: %s", ErrSymCycle, err)
	}

	// make sure to clean up everything
	if err := os.Remove("symTestA/linkToB.sym"); err != nil {
		t.Errorf("Could not remove path: %s", err)
	}

	if err := os.Remove("symTestB/linkToA.sym"); err != nil {
		t.Errorf("Could not remove path: %s", err)
	}

	if err := os.Remove("symTestA"); err != nil {
		t.Errorf("Could not remove path: %s", err)
	}

	if err := os.Remove("symTestB"); err != nil {
		t.Errorf("Could not remove path: %s", err)
	}

}

// TestSymlinkToFolder checks if we are successfully following symlinks to folders
func TestSymlinkToFolder(t *testing.T) {
	if err := os.MkdirAll("symTest/symTest2", 0700); err != nil {
		t.Errorf("Could not create tmpdir: %s", err)
	}

	if err := os.Symlink("symTest/symTest2", "symTmpfile.sym"); err != nil {
		t.Errorf("Could not create a symlink: %s", err)
	}

	// create a filepath from slash, because otherwise
	// our tests are going to fail, because the path matching will
	// not work correctly on Windows
	p := filepath.FromSlash("symTest/symTest2/symTmpfile")

	if err := ioutil.WriteFile(p, []byte("abc"), 0400); err != nil {
		t.Errorf("Could not write symTmpfile: %s", err)
	}

	result, err := RecordArtifacts([]string{"symTmpfile.sym"})
	if err != nil {
		t.Error(err)
	}

	expected := map[string]interface{}{
		p: map[string]interface{}{
			"sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("RecordArtifacts returned '(%s, %s)', expected '(%s, nil)'",
			result, err, expected)
	}

	// make sure to clean up everything
	if err := os.Remove("symTest/symTest2/symTmpfile"); err != nil {
		t.Errorf("Could not remove path symTest/symTest2/symTmpfile: %s", err)
	}

	if err := os.Remove("symTmpfile.sym"); err != nil {
		t.Errorf("Could not remove path symTest/symTest2/symTmpfile.sym: %s", err)
	}

	if err := os.Remove("symTest/symTest2"); err != nil {
		t.Errorf("Could not remove path symTest/symTest2: %s", err)
	}

	if err := os.Remove("symTest/"); err != nil {
		t.Errorf("Could not remove path symTest: %s", err)
	}
}

// This test provokes a symlink cycle
func TestSymlinkCycle(t *testing.T) {
	if err := os.Mkdir("symlinkCycle/", 0700); err != nil {
		t.Errorf("Could not create tmpdir: %s", err)
	}

	// we need to get the current working directory here, otherwise
	// os.Symlink() will create a wrong symlink
	dir, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}
	// create a cycle ./symlinkCycle/symCycle.sym -> ./symlinkCycle/
	if err := os.Symlink(dir+"/symlinkCycle", "symlinkCycle/symCycle.sym"); err != nil {
		t.Errorf("Could not create a symlink: %s", err)
	}

	// provoke "symlink cycle detected" error
	_, err = RecordArtifacts([]string{"symlinkCycle/symCycle.sym", "foo.tar.gz"})
	if !errors.Is(err, ErrSymCycle) {
		t.Errorf("We expected: %s, we got: %s", ErrSymCycle, err)
	}

	if err := os.Remove("symlinkCycle/symCycle.sym"); err != nil {
		t.Errorf("Could not remove path symlinkCycle/symCycle.sym: %s", err)
	}

	if err := os.Remove("symlinkCycle"); err != nil {
		t.Errorf("Could not remove path symlinkCycle: %s", err)
	}
}

func TestRecordArtifacts(t *testing.T) {
	// Test successfully record multiple artifacts including temporary subdir
	if err := os.Mkdir("tmpdir", 0700); err != nil {
		t.Errorf("Could not create tmpdir: %s", err)
	}
	if err := ioutil.WriteFile("tmpdir/tmpfile", []byte("abc"), 0400); err != nil {
		t.Errorf("Could not write tmpfile: %s", err)
	}
	result, err := RecordArtifacts([]string{"foo.tar.gz",
		"demo.layout.template", "tmpdir/tmpfile"})
	expected := map[string]interface{}{
		"foo.tar.gz": map[string]interface{}{
			"sha256": "52947cb78b91ad01fe81cd6aef42d1f6817e92b9e6936c1e5aabb7c98514f355",
		},
		"demo.layout.template": map[string]interface{}{
			"sha256": "019e121a1e0a34aecde0aebb642162b11db4248c781cb8119f81f592723a0424",
		},
		"tmpdir/tmpfile": map[string]interface{}{
			"sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("RecordArtifacts returned '(%s, %s)', expected '(%s, nil)'",
			result, err, expected)
	}
	if err := os.RemoveAll("tmpdir"); err != nil {
		t.Errorf("Could not remove tmpdir: %s", err)
	}

	// Test error by recording nonexistent artifact
	result, err = RecordArtifacts([]string{"file-does-not-exist"})
	if !os.IsNotExist(err) {
		t.Errorf("RecordArtifacts returned '(%s, %s)', expected '(nil, %s)'",
			result, err, os.ErrNotExist)
	}
}

func TestWaitErrToExitCode(t *testing.T) {
	// TODO: Find way to test/mock ExitError
	// Test exit code from error assessment
	parameters := []error{
		nil,
		errors.New(""),
		// &exec.ExitError{ProcessState: &os.ProcessState},
	}
	expected := []int{
		0,
		-1,
		// -1,
	}

	for i := 0; i < len(parameters); i++ {
		result := WaitErrToExitCode(parameters[i])
		if result != expected[i] {
			t.Errorf("WaitErrToExitCode returned %d, expected %d",
				result, expected[i])
		}
	}
}

func TestRunCommand(t *testing.T) {
	// Successfully run command and return metadata
	parameters := [][]string{
		{"sh", "-c", "true"},
		{"sh", "-c", "false"},
		{"sh", "-c", "printf out"},
		{"sh", "-c", "printf err >&2"},
	}
	expected := []map[string]interface{}{
		{"return-value": float64(0), "stdout": "", "stderr": ""},
		{"return-value": float64(1), "stdout": "", "stderr": ""},
		{"return-value": float64(0), "stdout": "out", "stderr": ""},
		{"return-value": float64(0), "stdout": "", "stderr": "err"},
	}
	for i := 0; i < len(parameters); i++ {
		result, err := RunCommand(parameters[i])
		if !reflect.DeepEqual(result, expected[i]) || err != nil {
			t.Errorf("RunCommand returned '(%s, %s)', expected '(%s, nil)'",
				result, err, expected[i])
		}
	}

	// Fail run command
	result, err := RunCommand([]string{"command-does-not-exist"})
	if result != nil || err == nil {
		t.Errorf("RunCommand returned '(%s, %s)', expected '(nil, *exec.Error)'",
			result, err)
	}
}

func TestInTotoRun(t *testing.T) {
	// Successfully run InTotoRun
	linkName := "Name"

	var validKey Key
	if err := validKey.LoadKey("carol", "ed25519", []string{"sha256", "sha512"}); err != nil {
		t.Error(err)
	}

	tablesCorrect := []struct {
		materialPaths []string
		productPaths  []string
		cmdArgs       []string
		key           Key
		result        Metablock
	}{
		{[]string{"demo.layout.template"}, []string{"foo.tar.gz"}, []string{"sh", "-c", "printf out; printf err >&2"}, validKey, Metablock{
			Signed: Link{
				Name: linkName,
				Type: "link",
				Materials: map[string]interface{}{
					"demo.layout.template": map[string]interface{}{
						"sha256": "019e121a1e0a34aecde0aebb642162b11db4248c781cb8119f81f592723a0424",
					},
				},
				Products: map[string]interface{}{
					"foo.tar.gz": map[string]interface{}{
						"sha256": "52947cb78b91ad01fe81cd6aef42d1f6817e92b9e6936c1e5aabb7c98514f355",
					},
				},
				ByProducts: map[string]interface{}{
					"return-value": float64(0), "stdout": "out", "stderr": "err",
				},
				Command:     []string{"sh", "-c", "printf out; printf err >&2"},
				Environment: map[string]interface{}{},
			},
			Signatures: []Signature{{
				KeyId: "be6371bc627318218191ce0780fd3183cce6c36da02938a477d2e4dfae1804a6",
				Sig:   "08a6c42b8433502f2869bb3dc73f8348f6b6f89e42bbc63f91a33e7171d762e138ed5d695fb83cebec958203e17b2285f95b198d758bc62cf30e1f7408d6c10c",
			}},
		},
		},
	}

	for _, table := range tablesCorrect {
		result, err := InTotoRun(linkName, table.materialPaths, table.productPaths, table.cmdArgs, table.key)
		if !reflect.DeepEqual(result, table.result) {
			t.Errorf("InTotoRun returned '(%s, %s)', expected '(%s, nil)'", result, err, table.result)
		} else {
			// we do not need to check if result == nil here, because our reflect.DeepEqual was successful
			if err := result.Dump(linkName + ".link"); err != nil {
				t.Errorf("Error while dumping link metablock to file")
			}
			var loadedResult Metablock
			if err := loadedResult.Load(linkName + ".link"); err != nil {
				t.Errorf("Error while loading link metablock from file")
			}
			if !reflect.DeepEqual(loadedResult, result) {
				t.Errorf("Dump and loading of signed Link failed. Loaded result: '%s', dumped result '%s'", loadedResult, result)
			} else {
				if err := os.Remove(linkName + ".link"); err != nil {
					t.Errorf("Removing created link file failed")
				}
			}
		}
	}

	// Run InToToRun with errors
	tablesInvalid := []struct {
		materialPaths []string
		productPaths  []string
		cmdArgs       []string
		key           Key
	}{
		{[]string{"material-does-not-exist"}, []string{""}, []string{"sh", "-c", "printf test"}, Key{}},
		{[]string{"demo.layout.template"}, []string{"product-does-not-exist"}, []string{"sh", "-c", "printf test"}, Key{}},
		{[]string{""}, []string{"/invalid-path/"}, []string{"sh", "-c", "printf test"}, Key{}},
		{[]string{}, []string{}, []string{"command-does-not-exist"}, Key{}},
		{[]string{"demo.layout.template"}, []string{"foo.tar.gz"}, []string{"sh", "-c", "printf out; printf err >&2"}, Key{
			KeyId:               "this-is-invalid",
			KeyIdHashAlgorithms: nil,
			KeyType:             "",
			KeyVal:              KeyVal{},
			Scheme:              "",
		}},
	}

	for _, table := range tablesInvalid {
		result, err := InTotoRun(linkName, table.materialPaths, table.productPaths, table.cmdArgs, table.key)
		if err == nil {
			t.Errorf("InTotoRun returned '(%s, %s)', expected error",
				result, err)
		}
	}
}