cask "qrouton" do
  version "0.4.0"
  sha256 "e2f12c3121071534b8fc10c91542b75d29afc66ee31faeccdf303ed4d7357a74"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.4.0/qrouton-0.4.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
