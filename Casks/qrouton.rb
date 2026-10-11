cask "qrouton" do
  version "0.13.3"
  sha256 "1bde32de2dfdf46849769094820278d3e926cc0f9ff89f00ccff71a679d021a4"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.13.3/qrouton-0.13.3-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
