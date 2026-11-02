import * as THREE from "three";

// Community 鲸鱼娘 / 蓝色大肥鱼 sprites (MIT). See web/brand/dafeiyu/NOTICE.md.

function radial(inner, outer) {
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 128;
  const g = canvas.getContext("2d");
  const ramp = g.createRadialGradient(64, 64, 8, 64, 64, 64);
  ramp.addColorStop(0, inner);
  ramp.addColorStop(1, outer);
  g.fillStyle = ramp;
  g.fillRect(0, 0, 128, 128);
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  return tex;
}

function loadTexture(url) {
  return new Promise((resolve, reject) => {
    const loader = new THREE.TextureLoader();
    loader.load(
      url,
      (tex) => {
        tex.colorSpace = THREE.SRGBColorSpace;
        tex.anisotropy = 4;
        tex.needsUpdate = true;
        resolve(tex);
      },
      undefined,
      reject
    );
  });
}

export function createWhale(canvas) {
  const renderer = new THREE.WebGLRenderer({
    canvas,
    antialias: true,
    alpha: true,
    premultipliedAlpha: false,
  });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.setClearColor(0x000000, 0);

  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(30, 1, 0.1, 40);
  camera.position.set(0, 0.2, 6.4);
  camera.lookAt(0, 0.1, 0);
  scene.add(new THREE.AmbientLight(0xffffff, 1.05));

  const group = new THREE.Group();
  scene.add(group);

  // Aspect ~768x832
  const aspect = 768 / 832;
  const h = 3.15;
  const w = h * aspect;
  const geo = new THREE.PlaneGeometry(w, h);

  const idleMat = new THREE.MeshBasicMaterial({
    transparent: true,
    depthWrite: false,
    side: THREE.DoubleSide,
    opacity: 1,
  });
  const happyMat = new THREE.MeshBasicMaterial({
    transparent: true,
    depthWrite: false,
    side: THREE.DoubleSide,
    opacity: 0,
  });

  const idleMesh = new THREE.Mesh(geo, idleMat);
  const happyMesh = new THREE.Mesh(geo.clone(), happyMat);
  idleMesh.position.z = 0.01;
  happyMesh.position.z = 0.02;
  group.add(idleMesh, happyMesh);

  const dots = [0, 1, 2].map(() => {
    const dot = new THREE.Mesh(
      new THREE.SphereGeometry(0.055, 12, 10),
      new THREE.MeshBasicMaterial({ color: 0x8fd4ff })
    );
    dot.visible = false;
    group.add(dot);
    return dot;
  });

  const shadow = new THREE.Mesh(
    new THREE.CircleGeometry(1.05, 40),
    new THREE.MeshBasicMaterial({
      map: radial("rgba(4,12,28,0.34)", "rgba(4,12,28,0)"),
      transparent: true,
      depthWrite: false,
    })
  );
  shadow.rotation.x = -Math.PI / 2;
  shadow.position.y = -1.72;
  scene.add(shadow);

  let mood = "idle";
  let bg = "studio";
  let talkMix = 0;
  let thinkMix = 0;
  let alive = true;
  let ready = false;
  const clock = new THREE.Clock();
  let elapsed = 0;

  Promise.all([
    loadTexture("/brand/dafeiyu/fish_idle.png"),
    loadTexture("/brand/dafeiyu/fish_happy.png"),
  ])
    .then(([idleTex, happyTex]) => {
      idleMat.map = idleTex;
      happyMat.map = happyTex;
      idleMat.needsUpdate = true;
      happyMat.needsUpdate = true;
      ready = true;
    })
    .catch((err) => console.warn("鲸鱼娘立绘加载失败", err));

  function frame() {
    if (!alive) return;
    const dt = Math.min(clock.getDelta(), 0.05);
    elapsed += dt;
    const talkTarget = mood === "speaking" ? 1 : 0;
    const thinkTarget = mood === "thinking" ? 1 : 0;
    talkMix += (talkTarget - talkMix) * Math.min(1, dt * 8);
    thinkMix += (thinkTarget - thinkMix) * Math.min(1, dt * 5);

    const bob = Math.sin(elapsed * 1.35);
    group.position.y = bob * 0.1;
    group.rotation.z = Math.sin(elapsed * 0.8) * 0.035;
    group.rotation.y = Math.sin(elapsed * 0.35) * 0.06;

    // PNGTuber-style: crossfade idle ↔ happy while speaking, plus a light bounce.
    const pulse = 1 + talkMix * (0.03 + 0.05 * Math.abs(Math.sin(elapsed * 11)));
    const thinkScale = 1 - thinkMix * 0.03;
    group.scale.setScalar(pulse * thinkScale);

    const happyOpacity = Math.min(1, talkMix * 1.15);
    happyMat.opacity = happyOpacity;
    idleMat.opacity = 1 - happyOpacity * 0.85;

    dots.forEach((dot, i) => {
      dot.visible = thinkMix > 0.35;
      const a = elapsed * 2.5 + (i * Math.PI * 2) / 3;
      dot.position.set(Math.cos(a) * 0.38, 1.72 + Math.sin(elapsed * 3 + i) * 0.05, 0.08);
    });

    shadow.visible = bg === "studio";
    shadow.scale.setScalar(1 - bob * 0.05);

    renderer.render(scene, camera);
    requestAnimationFrame(frame);
  }
  requestAnimationFrame(frame);

  function resize() {
    const w0 = canvas.clientWidth || canvas.parentElement?.clientWidth || 1;
    const h0 = canvas.clientHeight || canvas.parentElement?.clientHeight || 1;
    if (w0 < 2 || h0 < 2) return;
    const obs = document.documentElement.classList.contains("obs");
    const narrow = !obs && w0 < 860;
    camera.position.set(0, narrow ? 0.65 : 0.2, narrow ? 7.8 : 6.4);
    camera.lookAt(0, narrow ? -0.15 : 0.1, 0);
    camera.aspect = w0 / h0;
    camera.updateProjectionMatrix();
    renderer.setSize(w0, h0, false);
  }

  function setBackground(mode, chroma) {
    bg = mode;
    if (mode === "chroma" || mode === "magenta") {
      const hex = mode === "magenta" ? "#ff00ff" : chroma || "#00ff00";
      renderer.setClearColor(parseHex(hex), 1);
      canvas.style.background = hex;
    } else {
      renderer.setClearColor(0x000000, 0);
      canvas.style.background = "transparent";
    }
  }

  resize();
  return {
    resize,
    setMood(next) {
      mood = next || "idle";
    },
    setBackground,
    dispose() {
      alive = false;
      renderer.dispose();
    },
    get ready() {
      return ready;
    },
  };
}

function parseHex(hex) {
  let h = String(hex || "").trim().replace("#", "");
  if (h.length === 3) h = h.split("").map((c) => c + c).join("");
  const n = Number.parseInt(h, 16);
  return Number.isFinite(n) ? n : 0x00ff00;
}
