cask "qrouton" do
  version "0.13.1"
  sha256 "85ac241adceeb4675b806c14f936be15334d3dedbdb5f30073af788d31d52521"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.13.1/qrouton-0.13.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
