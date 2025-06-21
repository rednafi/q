class Q < Formula
  desc "A CLI tool for interacting with AI models"
  homepage "https://github.com/rednafi/q"
  version "0.0.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/rednafi/q/releases/download/v0.0.0/q_Darwin_arm64.tar.gz"
      sha256 "placeholder"
    else
      url "https://github.com/rednafi/q/releases/download/v0.0.0/q_Darwin_x86_64.tar.gz"
      sha256 "placeholder"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/rednafi/q/releases/download/v0.0.0/q_Linux_arm64.tar.gz"
      sha256 "placeholder"
    else
      url "https://github.com/rednafi/q/releases/download/v0.0.0/q_Linux_x86_64.tar.gz"
      sha256 "placeholder"
    end
  end

  def install
    bin.install "q"
  end

  test do
    system "#{bin}/q", "--help"
  end
end
