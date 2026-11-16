cask "qrouton" do
  version "0.17.2"
  sha256 "1d8b8013a8bbe0c75dfb8f5fb1763e1356b0e8e7cbaaefd22cfb85bf91921f31"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.17.2/qrouton-0.17.2-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
