cask "qrouton" do
  version "0.12.1"
  sha256 "ae35cdc17ba9cd6e1392cb9e210b81954e6cb80d3e54ed42a58ab2456a193636"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.12.1/qrouton-0.12.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
