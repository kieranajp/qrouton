cask "qrouton" do
  version "0.12.0"
  sha256 "5e412a6e29ef84932c9396685fab4713dc0c7ce75a961cd9ae176e55aa82cfcc"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.12.0/qrouton-0.12.0-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
