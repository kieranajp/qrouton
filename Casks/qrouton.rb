cask "qrouton" do
  version "0.18.0"
  sha256 "fc52a1429db1ff0988d4b1331b415d77536c7e3d52215fe37f3b314fbcb3a383"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.18.0/qrouton-0.18.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
