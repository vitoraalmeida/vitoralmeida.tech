from __future__ import annotations

import hashlib
import io
import os
import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("deploy_static.py")


class DeployStaticTest(unittest.TestCase):
    """Protege o contrato externo do script usando deployments isolados reais."""

    def setUp(self) -> None:
        """Cria uma árvore descartável para que cada teste não compartilhe estado."""
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / "incoming").mkdir()
        bootstrap = self.root / "releases" / "bootstrap"
        bootstrap.mkdir(parents=True)
        (bootstrap / "index.html").write_text("bootstrap", encoding="utf-8")
        (self.root / "current").symlink_to("releases/bootstrap")

    def create_package(
        self,
        commit: str,
        *,
        unsafe_path: str | None = None,
        symbolic_link: bool = False,
    ) -> None:
        """Produz pacote e checksum controlados para exercitar validações específicas."""
        package = self.root / "incoming" / f"site-{commit}.tar.gz"
        files = {
            "index.html": b"index",
            "404.html": b"not found",
            "styles/global.css": b"body {}",
            "robots.txt": b"User-agent: *",
            "sitemap.xml": b'<?xml version="1.0"?><urlset/>',
            "feed.xml": b'<?xml version="1.0"?><rss/>',
            "og-image.png": b"\x89PNG",
        }
        with tarfile.open(package, "w:gz") as archive:
            for name, content in files.items():
                info = tarfile.TarInfo(name)
                info.size = len(content)
                archive.addfile(info, io.BytesIO(content))
            if unsafe_path:
                info = tarfile.TarInfo(unsafe_path)
                info.size = 3
                archive.addfile(info, io.BytesIO(b"bad"))
            if symbolic_link:
                info = tarfile.TarInfo("unsafe-link")
                info.type = tarfile.SYMTYPE
                info.linkname = "/etc/passwd"
                archive.addfile(info)

        digest = hashlib.sha256(package.read_bytes()).hexdigest()
        package.with_suffix(package.suffix + ".sha256").write_text(
            f"{digest}  {package.name}\n", encoding="utf-8"
        )
        (self.root / "incoming" / "deploy_static.py").write_text("uploaded", encoding="utf-8")

    def run_deploy(self, commit: str, healthcheck_url: str) -> subprocess.CompletedProcess[str]:
        """Executa a CLI como processo para testar o mesmo limite usado na produção."""
        environment = os.environ.copy()
        environment.update(
            APP_ROOT=str(self.root),
            HEALTHCHECK_URL=healthcheck_url,
            KEEP_RELEASES="5",
        )
        return subprocess.run(
            ["python3", str(SCRIPT), commit],
            check=False,
            capture_output=True,
            text=True,
            env=environment,
        )

    def test_success_and_idempotent_redeployment(self) -> None:
        """Garante publicação completa e repetição segura do mesmo commit."""
        commit = "1" * 40
        self.create_package(commit)
        result = self.run_deploy(commit, (self.root / "current" / "index.html").as_uri())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(os.readlink(self.root / "current"), f"releases/{commit}")
        self.assertEqual((self.root / "releases" / commit).stat().st_mode & 0o777, 0o755)
        self.assertEqual((self.root / "releases" / commit / "index.html").stat().st_mode & 0o777, 0o644)

        self.create_package(commit)
        result = self.run_deploy(commit, (self.root / "current" / "index.html").as_uri())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(os.readlink(self.root / "current"), f"releases/{commit}")

    def test_health_check_failure_rolls_back(self) -> None:
        """Garante que uma versão reprovada nunca permaneça apontada por current."""
        commit = "2" * 40
        self.create_package(commit)
        result = self.run_deploy(commit, "http://127.0.0.1:1/")
        self.assertEqual(result.returncode, 1)
        self.assertIn("health check failed", result.stderr)
        self.assertEqual(os.readlink(self.root / "current"), "releases/bootstrap")
        self.assertFalse((self.root / "releases" / commit).exists())

    def test_checksum_mismatch_is_rejected(self) -> None:
        """Garante que conteúdo diferente do artifact aprovado não seja extraído."""
        commit = "3" * 40
        self.create_package(commit)
        checksum = self.root / "incoming" / f"site-{commit}.tar.gz.sha256"
        checksum.write_text(f"{'0' * 64}  site-{commit}.tar.gz\n", encoding="utf-8")
        result = self.run_deploy(commit, (self.root / "current" / "index.html").as_uri())
        self.assertEqual(result.returncode, 1)
        self.assertIn("checksum mismatch", result.stderr)
        self.assertEqual(os.readlink(self.root / "current"), "releases/bootstrap")

    def test_unsafe_archive_path_is_rejected(self) -> None:
        """Garante que uma entrada com path traversal não escreva fora da release."""
        commit = "4" * 40
        self.create_package(commit, unsafe_path="../outside.txt")
        result = self.run_deploy(commit, (self.root / "current" / "index.html").as_uri())
        self.assertEqual(result.returncode, 1)
        self.assertIn("unsafe archive path", result.stderr)
        self.assertFalse((self.root / "outside.txt").exists())
        self.assertEqual(os.readlink(self.root / "current"), "releases/bootstrap")

    def test_symbolic_link_is_rejected(self) -> None:
        """Garante que links no archive não contornem o isolamento da extração."""
        commit = "5" * 40
        self.create_package(commit, symbolic_link=True)
        result = self.run_deploy(commit, (self.root / "current" / "index.html").as_uri())
        self.assertEqual(result.returncode, 1)
        self.assertIn("unsupported entry type", result.stderr)
        self.assertEqual(os.readlink(self.root / "current"), "releases/bootstrap")


if __name__ == "__main__":
    unittest.main()
