cask "qrouton" do
  version "0.2.0"
  sha256 "f48fe67d6a1e4c86eafa4fcc4600f9ad174bacf2a6be6578f6340c9d2501c72a"

  url "https://github.com/kieranajp/qrouton/releases/download/v#{version}/qrouton-#{version}-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
