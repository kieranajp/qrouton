cask "qrouton" do
  version "0.14.0"
  sha256 "6d07eae307adef5a86c402c4d483628579c2208f3d0d3cba26979b967f4f2688"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.14.0/qrouton-0.14.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
