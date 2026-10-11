cask "qrouton" do
  version "0.13.0"
  sha256 "3d455fb121c3e25de5bd555ce6d9711579e30a76758a5c783e8c90915c70966b"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.13.0/qrouton-0.13.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
