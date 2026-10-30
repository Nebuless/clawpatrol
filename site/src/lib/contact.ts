export const assessmentEmail =
  "mailto:pilot@nebuless.cloud?subject=" +
  encodeURIComponent("Discuss an on-site AI toolchain assessment") +
  "&body=" +
  encodeURIComponent(
    [
      "Hello Nebuless,",
      "",
      "We would like to discuss an on-site AI toolchain assessment.",
      "Organization:",
      "Tools and workflows to review:",
      "Who should join the conversation:",
      "",
      "Please help us agree scope and timing.",
    ].join("\r\n"),
  );
