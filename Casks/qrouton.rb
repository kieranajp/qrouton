cask "qrouton" do
  version "0.10.2"
  sha256 "df0447fea95201449673d3d7de060887c1be7bb5f6e854ed84e61ecc989699dc"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.10.2/qrouton-0.10.2-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
