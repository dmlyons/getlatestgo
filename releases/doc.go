// Package releases is the domain layer of getlatestgo: it fetches and parses the
// official Go release feed ([DefaultURL], the golang.org/dl JSON schema mapped
// onto [GoRelease] and [GoFile]) and selects the release and file to download.
// It depends on the download package only for [download.RetryClient], which
// [Fetch] uses to retrieve the feed.
//
// The feed lists releases newest first. [FindRelease] with an empty version
// returns the first stable release (the newest); with a version such as
// "go1.22.5" it requires an exact match, unstable releases included.
// [StableVersions] keeps feed order. [FindFile] picks the file whose os and
// arch match exactly (callers pass runtime.GOOS and runtime.GOARCH).
//
// [GoFile.DownloadURL] is [DownloadBase] plus the file name.
// [GoFile.LocalPath] is the file name inside [os.TempDir]; because that name
// encodes version, OS and arch, download.EnsureFile can safely reuse a verified
// file found there.
package releases
