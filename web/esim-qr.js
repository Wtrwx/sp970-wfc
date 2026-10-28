/* SPDX-License-Identifier: AGPL-3.0-or-later */
(function (root) {
  "use strict";

  var decoderPromise;

  function activationCode(value) {
    var code = String(value || "").trim();
    if (/^1\$/.test(code)) code = "LPA:" + code;
    if (!/^LPA:1\$/i.test(code) || code.length > 1024 || /[\x00-\x20\x7f]/.test(code)) return null;
    var parts = code.slice(4).split("$");
    if (parts.length < 3 || parts.length > 5 || !parts[1] || /[\/@?#]/.test(parts[1])) return null;
    try {
      var address = new URL("https://" + parts[1]);
      if (!address.hostname || address.username || address.password || address.pathname !== "/") return null;
    } catch (_) { return null; }
    return "LPA:" + code.slice(4);
  }

  function loadDecoder() {
    if (root.jsQR) return Promise.resolve(root.jsQR);
    if (!decoderPromise) {
      decoderPromise = new Promise(function (resolve, reject) {
        var script = document.createElement("script");
        script.src = "/jsqr.js?v=1.4.0";
        script.onload = function () {
          if (root.jsQR) resolve(root.jsQR);
          else { decoderPromise = null; reject(new Error("decoder")); }
        };
        script.onerror = function () { decoderPromise = null; script.remove(); reject(new Error("decoder")); };
        document.head.appendChild(script);
      });
    }
    return decoderPromise;
  }

  function Scanner(options) {
    this.options = options;
    this.modal = document.getElementById("euicc-qr-modal");
    this.video = document.getElementById("euicc-qr-video");
    this.preview = document.getElementById("euicc-qr-preview");
    this.status = document.getElementById("euicc-qr-status");
    this.camera = document.getElementById("euicc-qr-camera");
    this.image = document.getElementById("euicc-qr-image");
    this.file = document.getElementById("euicc-qr-file");
    this.closeButton = document.getElementById("euicc-qr-close");
    this.canvas = document.createElement("canvas");
    this.generation = 0;
    this.stream = null;
    this.cameraPending = false;
    this.frame = null;
    this.hint = "euicc.qrChoose";
    this.canUseCamera = !!(root.isSecureContext && navigator.mediaDevices && navigator.mediaDevices.getUserMedia);
    var self = this;
    this.camera.addEventListener("click", function () { self.startCamera(); });
    this.image.addEventListener("click", function () {
      self.generation++;
      self.stopCamera();
      self.setLoading(false);
      self.file.click();
    });
    this.file.addEventListener("change", function () {
      var file = self.file.files && self.file.files[0];
      self.file.value = "";
      if (file && !self.modal.hidden) self.readImage(file);
    });
    this.closeButton.addEventListener("click", function () { self.close(); });
    this.modal.addEventListener("click", function (event) { if (event.target === self.modal) self.close(); });
    document.addEventListener("keydown", function (event) {
      if (self.modal.hidden) return;
      if (event.key === "Escape") { event.preventDefault(); self.close(); }
      if (event.key === "Tab") {
        var buttons = [self.closeButton, self.camera, self.image].filter(function (button) { return !button.hidden && !button.disabled; });
        var first = buttons[0], last = buttons[buttons.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
      }
    });
    document.addEventListener("visibilitychange", function () {
      if (document.hidden && (self.stream || self.cameraPending)) {
        self.generation++;
        self.stopCamera();
        self.setLoading(false);
        self.setHint("euicc.qrPaused");
      }
    });
    root.addEventListener("pagehide", function () { self.close(); });
    this.syncText();
  }

  Scanner.prototype.syncText = function () {
    this.camera.hidden = !this.canUseCamera;
    this.camera.textContent = this.options.t("euicc.qrCamera");
    this.image.textContent = this.options.t(this.canUseCamera ? "euicc.qrImage" : "euicc.qrPhotoImage");
    this.status.textContent = this.options.t(this.hint);
  };

  Scanner.prototype.setHint = function (key) {
    this.hint = key;
    this.status.textContent = this.options.t(key);
  };

  Scanner.prototype.setLoading = function (loading) {
    this.camera.disabled = loading;
    this.image.disabled = loading;
  };

  Scanner.prototype.open = function () {
    this.generation++;
    this.returnFocus = document.activeElement;
    this.modal.hidden = false;
    this.setHint(this.canUseCamera ? "euicc.qrChoose" : "euicc.qrChoosePhoto");
    this.setLoading(false);
    this.syncText();
    (this.canUseCamera ? this.camera : this.image).focus();
  };

  Scanner.prototype.stopCamera = function () {
    if (this.frame !== null) root.cancelAnimationFrame(this.frame);
    this.frame = null;
    if (this.stream) this.stream.getTracks().forEach(function (track) { track.stop(); });
    this.stream = null;
    this.cameraPending = false;
    this.video.pause();
    this.video.srcObject = null;
    this.preview.hidden = true;
  };

  Scanner.prototype.close = function () {
    this.generation++;
    this.stopCamera();
    var wasOpen = !this.modal.hidden;
    this.modal.hidden = true;
    this.setLoading(false);
    this.file.value = "";
    if (wasOpen && this.returnFocus && this.returnFocus.isConnected) this.returnFocus.focus();
  };

  Scanner.prototype.accept = function (value) {
    var code = activationCode(value);
    if (!code) { this.setHint("euicc.qrNotActivation"); return false; }
    this.close();
    this.options.onResult(code);
    return true;
  };

  Scanner.prototype.decode = function (source, width, height, scale, decoder) {
    this.canvas.width = Math.max(1, Math.round(width * scale));
    this.canvas.height = Math.max(1, Math.round(height * scale));
    var context = this.canvas.getContext("2d", { willReadFrequently: true });
    context.drawImage(source, 0, 0, this.canvas.width, this.canvas.height);
    var pixels = context.getImageData(0, 0, this.canvas.width, this.canvas.height);
    return decoder(pixels.data, pixels.width, pixels.height, { inversionAttempts: "attemptBoth" });
  };

  Scanner.prototype.startCamera = function () {
    if (!this.canUseCamera) return;
    this.stopCamera();
    var self = this, generation = ++this.generation;
    this.cameraPending = true;
    this.camera.disabled = true;
    this.setHint("euicc.qrCameraOpening");
    navigator.mediaDevices.getUserMedia({ audio: false, video: { facingMode: { ideal: "environment" }, width: { ideal: 1280 }, height: { ideal: 720 } } }).then(function (stream) {
      if (generation !== self.generation || self.modal.hidden) {
        stream.getTracks().forEach(function (track) { track.stop(); });
        return;
      }
      self.stream = stream;
      self.cameraPending = false;
      return loadDecoder().then(function (decoder) {
        if (generation !== self.generation || self.modal.hidden) return;
        self.video.srcObject = stream;
        self.preview.hidden = false;
        return self.video.play().then(function () {
          if (generation !== self.generation || self.modal.hidden) return;
          self.setHint("euicc.qrAim");
          var last = 0;
          function scan(time) {
            if (generation !== self.generation || self.modal.hidden) return;
            if (time - last >= 180 && self.video.readyState >= 2 && self.video.videoWidth) {
              last = time;
              try {
                var width = self.video.videoWidth, height = self.video.videoHeight;
                var result = self.decode(self.video, width, height, Math.min(1, 960 / width, 720 / height), decoder);
                if (result && self.accept(result.data)) return;
              } catch (_) {
                self.stopCamera();
                self.setLoading(false);
                self.setHint("euicc.qrCameraError");
                return;
              }
            }
            self.frame = root.requestAnimationFrame(scan);
          }
          self.frame = root.requestAnimationFrame(scan);
        });
      });
    }).catch(function () {
      if (generation !== self.generation || self.modal.hidden) return;
      self.stopCamera();
      self.setLoading(false);
      self.setHint("euicc.qrCameraError");
    });
  };

  Scanner.prototype.readImage = function (file) {
    if ((file.type && !/^image\//.test(file.type)) || file.size > 20 * 1024 * 1024) {
      this.setHint("euicc.qrImageType");
      return;
    }
    this.stopCamera();
    var self = this, generation = ++this.generation;
    this.setLoading(true);
    this.setHint("euicc.qrReading");
    var url = URL.createObjectURL(file);
    var image = new Image();
    var loaded = new Promise(function (resolve, reject) {
      image.onload = function () { resolve(image); };
      image.onerror = reject;
      image.src = url;
    });
    Promise.all([loaded, loadDecoder()]).then(function (values) {
      if (generation !== self.generation || self.modal.hidden) return;
      var width = image.naturalWidth, height = image.naturalHeight, largest = Math.max(width, height);
      if (!width || !height) throw new Error("image");
      var scales = [Math.min(1, 1600 / largest), Math.min(2, 2600 / largest)];
      for (var i = 0; i < scales.length; i++) {
        var result = self.decode(image, width, height, scales[i], values[1]);
        if (result) { self.accept(result.data); return; }
      }
      self.setHint("euicc.qrNotFound");
    }).catch(function () {
      if (generation === self.generation && !self.modal.hidden) self.setHint("euicc.qrReadError");
    }).finally(function () {
      URL.revokeObjectURL(url);
      if (generation === self.generation) self.setLoading(false);
    });
  };

  var api = { create: function (options) { return new Scanner(options); }, activationCode: activationCode };
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.WfcEsimQR = api;
}(typeof window === "object" ? window : globalThis));
