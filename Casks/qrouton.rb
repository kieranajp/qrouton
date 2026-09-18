cask "qrouton" do
  version "0.9.3"
  sha256 "0436c9a530d9ef50348e9c5ac856bdcfc9cef0980979a3a352752f98c4e13669"

  url "https://github.com/kieranajp/qrouton/releases/download/v0.9.3/qrouton-0.9.3-macos-universal.zip"
  name "qrouton"
  desc "Multi-repository workspace manager for coding agents"
  homepage "https://github.com/kieranajp/qrouton"

  depends_on macos: :monterey

  app "qrouton.app"
  binary "#{appdir}/qrouton.app/Contents/MacOS/qrouton"
end
