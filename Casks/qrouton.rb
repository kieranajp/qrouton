cask "qrouton" do
  version "0.9.2"
  sha256 "fcea2f9a2ad3d7f0cb666ce91d5a30819011a98bb1493969dbc0657c943445e2"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.9.2/qrouton-0.9.2-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
