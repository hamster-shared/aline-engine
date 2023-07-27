package action

import (
	"gotest.tools/v3/assert"
	"testing"
)

func TestAnalyzeURL(t *testing.T) {

	output := "\nDeploying all canisters.\nAll canisters have already been created.\nBuilding canisters...\nInstalling canisters...\nModule hash 1286960c50eb7a773cfb5fdd77cc238588f39e21f189cc3eb0f35199a99b9c7e is already installed.\nUploading assets to asset canister...\nFetching properties for all assets in the canister.\nStarting batch.\nStaging contents of new and changed assets in batch 5:\n  /index.html (747 bytes) sha aaeffc9d9e27ba284c3cd3faf08b7eaabab54fc9a2f786dd45a66909538b17ee is already installed\n  /index.html (gzip) (401 bytes) sha 51f51ab0c4c0fb9ad2d845944947ea3c28048f0444d5e5cc302d4d7385419d1f is already installed\n  /js/app.54849f11.js (4590 bytes) sha e9b8e868f5e7c0f11de83b3b472b3ffa077cbe81a4bc1f3c41c7354cb7562a2f is already installed\n  /js/app.54849f11.js (gzip) (1669 bytes) sha 6e874de68e6f4729f18a840584288e286e1c43ae2a713fc86327126e9ebde932 is already installed\n  /js/chunk-vendors.61a12961.js.map (714279 bytes) sha d4728f0a7c4dfd61bd0c086fa22a2e0d009f2ea564fb24c6883c106686302d1c is already installed\n  /js/chunk-vendors.61a12961.js.map (gzip) (186898 bytes) sha a1afa1bd24b63bd026806a5be1367550e738982f4f5d3a2d0fd156499d1dfaa5 is already installed\n  /favicon.ico (4286 bytes) sha db74ab0b78338c1f778f8398c45f4103c99aea0e845a3118a7750b4eeafd3445 is already installed\n  /css/app.fb0c6e1c.css (343 bytes) sha c419a17abd7e202d67167d2bf1b08feb6dd3f23e08c6432acd7230599d44a520 is already installed\n  /css/app.fb0c6e1c.css (gzip) (233 bytes) sha 959696c0f136abe41b4af21b8a6436e6230d84c7b674f63d3179dae018695696 is already installed\n  /js/app.54849f11.js.map (14028 bytes) sha b75ecfa3893a0420624f512bd3cd758ec6c005a716ad0608112fa2f1a6644df6 is already installed\n  /js/app.54849f11.js.map (gzip) (4678 bytes) sha 63c613de6a90f88dab860930c310612219ad1e72d7b88d58afc95f10864fdd05 is already installed\n  /img/logo.82b9c7a5.png (6849 bytes) sha 03d6d6da2545d3b3402855b8e721b779abaa87d113e69d9329ea6ea6325a83ce is already installed\n  /js/chunk-vendors.61a12961.js (91540 bytes) sha 8d8e9adcd9d767f4ff3fb4482571043653ee5a0e9ea53c9009b226ce43080a3d is already installed\n  /js/chunk-vendors.61a12961.js (gzip) (34446 bytes) sha 37fc4daae92c7dbb13249cfa1eb7329a752d7411bfad0e8ff80163f6aab04fcf is already installed\nCommitting batch.\nDeployed canisters.\nURLs:\n  Frontend canister via browser\n    vuejs: http://127.0.0.1:4943/?canisterId=dmalx-m4aaa-aaaaa-qaanq-cai\n"

	result := analyzeURL(output)

	assert.Equal(t, 1, len(result), "url length != 1")
	assert.Equal(t, "http://127.0.0.1:4943/?canisterId=dmalx-m4aaa-aaaaa-qaanq-cai", result["vuejs"])
}
