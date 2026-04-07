[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Tue Apr  7 17:42:35 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,css,js,mjs,yml,yaml --dryness --by-file --wide
scc --exclude-dir .git --include-ext go,css,js,mjs,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                     12       377       46         9      322          0             0.00
(ULOC)                                            237
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.github/workflows/pages.yml                        62        6         0       56          0             0.00
action.yml                                         51        6         1       44          0             0.00
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.github/workflows/lint-go.yml                      36        7         2       27          0             0.00
.pre-commit-config.yaml                            32        1         1       30          0             0.00
.github/workflows/lint-css.yml                     27        3         0       24          0             0.00
.github/workflows/lint-js.yml                      26        6         0       20          0             0.00
~rkflows/validate-actions-and-workflows.yml        24        2         0       22          0             0.00
.github/actions/upload-pages/action.yml            24        4         1       19          0             0.00
.github/actions/setup-node/action.yml              17        1         0       16          0             0.00
.golangci.yml                                      16        0         2       14          0             0.00
.github/dependabot.yml                             12        1         1       10          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                        6      1687      125        62     1500        231           104.25
(ULOC)                                            980
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
main_test.go                                      640       27         2      611         67            10.97
main.go                                           446       53        48      345         75            21.74
tree_test.go                                      227        7         0      220         28            12.73
tree.go                                           190       32        11      147         32            21.77
flags_test.go                                     149        4         0      145         22            15.17
flags.go                                           35        2         1       32          7            21.88
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
JavaScript                                5       323       34        49      240         13            25.55
(ULOC)                                            247
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
demo/render.js                                    154       22        37       95          0             0.00
demo/helpers.js                                   111        5        11       95          7             7.37
validate_actions_and_workflows.js                  39        6         0       33          6            18.18
demo/eslint.config.js                              15        1         0       14          0             0.00
stylelint.config.mjs                                4        0         1        3          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
CSS                                       2       254       38         1      215          0             0.00
(ULOC)                                            164
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
css/style.css                                     135       18         0      117          0             0.00
css/tree.css                                      119       20         1       98          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    25      2641      243       121     2277        244           129.80
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      1621
DRYness %                                        0.61
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $64,095
Estimated Schedule Effort (organic) 4.84 months
Estimated People Required (organic) 1.18
Processed 81125 bytes, 0.081 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
