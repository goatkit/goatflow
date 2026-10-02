package loader

// knownShippedBundledHashes lists, per bundled file, the sha256 of every
// version the released images v0.6.5 through v0.9.0 shipped (identical for
// linux/amd64 and linux/arm64). Those releases wrote no bundled manifest, so
// these hashes are how SyncBundledPlugins recognises an untouched bundled copy
// in a plugin directory they populated. Releases from v0.10.0 on record what
// they install in the manifest; this table never needs new entries.
var knownShippedBundledHashes = map[string][]string{
	"hello-wasm/build.sh": {
		"7db4df50a8aaad8529072a0b71efe77007468525505e2eb3b871d6568728a6c2",
	},
	"hello-wasm/hello.wasm": {
		"49feba2cac9e933e2eb907161ef668c7a925dd9ee512c1aaf355bfb815063206",
		"6399d9acc8a9ecb749acd5091a5c1da854c50e9f6ab44eef79a5d5b545e0c1f1",
	},
	"hello-wasm/main.go": {
		"cd46283c4ed7cdf274c9a4de8f931ac5f73dcc8cc1888a4cc216e5cf0323da6b",
		"f84299b23a9691034a27881083f408cfc04761ab479babd1557297e3219168d0",
	},
	"stats/README.md": {
		"8fb4f56d13dfec49c400e3e5442a7a1634990b0cd31633b02354c5f5947693e9",
	},
	"stats/build.sh": {
		"7d58f64561b1e0cff31330917696b3db0527596b56a44b11cc21bd2f8b0dd66d",
	},
	"stats/main.go": {
		"33a5c207a7b2102eb63d530a560690d75f0962d91de4590c65a42f263f4aebf1",
		"425eceaabab7395a5fd5fbe2c58d3b37fbc727c24c4e263f0323e24c42280329",
		"61b0b40df301da95d6e62a9c572b414cfe94d33c7e7a07a8fa0b59437b031f2f",
		"cd8c051b643a55dfe4c659d967cab4899306736aea6de794ce9fa618e7b23915",
		"f7327950838e61dfd4687018ba7adfac7586c954797d287bc8535ed190ce1e38",
	},
	"stats/stats.wasm": {
		"3f5c5b1e71d11bb25d28c8777e449e0736a8a98d68a7d5ad207ebead4b511239",
		"6fb79f8ed29d1f66572e410e7e7fdd438523bfcace3c1e749bb3519fc04db686",
		"7f026e493467d5cc752616beb2220036213cfb56e93ca8aaaf74d98cb982af5f",
		"947bef5ca24566067dabab90f3dd17056fa59fdd3fd0f359b5ced0633e45557e",
		"9f98c1511ee74f4a473249d13288ee664bb1ae97f4aab3b6574eb4ce8018e8ac",
	},
	"test-hostapi-wasm/main.go": {
		"25c3dfca1a728a70e6734cce78d7bb330e6dcef31d40c631b4928e53b15f16c6",
		"7664116781bb96fb52aacf664c595ee250118386c7c6f7e33f4ab1054f399c91",
		"e4d483c86de2c66c6bfc9088136e163de9ca05415692347031347f05d322b94a",
	},
	"test-hostapi-wasm/test-hostapi.wasm": {
		"5fd44373bd34d3852014603335bbe811d8a4213f2498260a7e927d75fd2f6414",
		"b13ab19229f6ebcd6013a191b7070e7433261f5b1b35d32d2ce0bf07ec2125e9",
	},
}
