cask "qrouton" do
  version "0.8.0"
  sha256 "536e921ca3d57df2db4907007c2c51941e7d223031b38ac3c335d65d9d14be78"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.8.0/qrouton-0.8.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
