cask "qrouton" do
  version "0.11.1"
  sha256 "cd7fe2a50f43922bce5b714534dc16e16d0adb5a7db1f03881110a08be68aed2"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.11.1/qrouton-0.11.1-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
