package vm

import "testing"

func TestArtifactURLUsesPathStyleS3EndpointByDefault(t *testing.T) {
	t.Setenv("K2_KAIROS_IMAGE_BASE_URL", "")

	const key = "latest/ubuntu-26.04-amd64-qemu-k8s/manifest.json"
	want := "https://s3.us-west-2.amazonaws.com/io.wyvernzora.k2.images/" + key
	if got := artifactURL(key); got != want {
		t.Fatalf("artifactURL(%q) = %q, want %q", key, got, want)
	}
}
