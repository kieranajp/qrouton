cask "qrouton" do
  version "0.11.0"
  sha256 "d16ae16bea4768a93f613cbf63b5bcfdd36b9cd63cad8ffeacb802fe094869be"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.11.0/qrouton-0.11.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
