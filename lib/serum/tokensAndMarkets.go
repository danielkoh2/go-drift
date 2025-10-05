package serum

var PROGRAM_LAYOUT_VERSIONS = map[string]int{
	"4ckmDgGdxQoPDLUkDT3vHgSAkzA3QRdNq5ywwY4sUSJn": 1,
	"BJ3jrUzddfuSrZHXSCxMUUQsjKEyLmuuyZebkcaFp2fg": 1,
	"EUqojwWA2rd19FZrzeBncJsm38Jm1hEhE3zsmX3bRc2o": 2,
	"9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin": 3,
}

func GetLayoutVersion(programId string) int {
	version, exists := PROGRAM_LAYOUT_VERSIONS[programId]
	if exists {
		return version
	}
	return 3
}
