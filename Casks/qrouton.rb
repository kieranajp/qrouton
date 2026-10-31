cask "qrouton" do
  version "0.17.0"
  sha256 "a8a0f5f4b072fe0020695d15c584f8634b4dc32bb666c969c47c2b9bd204e9e1"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.17.0/qrouton-0.17.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
