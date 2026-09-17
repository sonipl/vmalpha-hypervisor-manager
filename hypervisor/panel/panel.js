"use strict";
document.querySelectorAll("[data-page]").forEach((button) => {
  button.addEventListener("click", () => cockpit.jump(button.dataset.page));
});
cockpit.spawn(["hostname"], { err: "message" })
  .done((value) => { document.getElementById("hostname").textContent = value.trim(); })
  .fail(() => { document.getElementById("hostname").textContent = "Unavailable"; });
cockpit.spawn(["test", "-c", "/dev/kvm"], { err: "message" })
  .done(() => { document.getElementById("kvm").textContent = "KVM device detected"; })
  .fail(() => { document.getElementById("kvm").textContent = "No KVM device — check virtualization settings"; });
