# djinni-bot-go: Test-Apply Flow with Detailed Logging Plan

## TODOs
- [x] 1. Implement a live test-apply command in the CLI (`career-ops pipeline test-apply`) that runs a single end-to-end flow from scanning to PDF/cover letter generation and application submission, using a dedicated log file (`logs/test-apply.log`) for extremely verbose step-by-step trace output.
- [x] 2. Add verification test/run script to Makefile, document the command in README.md, and verify the output log structure.

## Final Verification Wave
- [x] F1. Automated checks (build and tests pass)
- [x] F2. Manual check of the generated `logs/test-apply.log` file verifying detailed output
