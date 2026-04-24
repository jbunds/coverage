[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Fri Apr 24 03:08:23 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,css,js,mjs,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                     12       436       51        36      349          0             0.00
(ULOC)                                            275
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.github/workflows/pages.yml                        82        7        13       62          0             0.00
action.yml                                         64        7         8       49          0             0.00
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.pre-commit-config.yaml                            37        1         2       34          0             0.00
.github/workflows/lint-go.yml                      35        7         2       26          0             0.00
.github/actions/upload-pages/action.yml            30        5         6       19          0             0.00
.github/workflows/lint-js.yml                      26        6         0       20          0             0.00
.github/workflows/lint-css.yml                     26        3         0       23          0             0.00
~rkflows/validate-actions-and-workflows.yml        24        2         0       22          0             0.00
.golangci.yml                                      23        1         2       20          0             0.00
.github/dependabot.yml                             20        1         1       18          0             0.00
.github/actions/setup-node/action.yml              19        2         1       16          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                       10      2818      254       115     2449        411           167.56
(ULOC)                                           1549
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
main_test.go                                      740       32         2      706         82            11.61
main.go                                           670       91        63      516        142            27.52
progress/progress_test.go                         349       25         0      324         36            11.11
progress/progress.go                              274       34        38      202         34            16.83
tree.go                                           249       42         9      198         52            26.26
tree_test.go                                      249        7         0      242         28            11.57
flags_test.go                                     148        3         0      145         17            11.72
progress/examples/fractional/main.go               56       10         1       45          8            17.78
progress/examples/weight-based/main.go             43        8         1       34          3             8.82
flags.go                                           40        2         1       37          9            24.32
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
JavaScript                                5       335       33        51      251          9            20.03
(ULOC)                                            249
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
demo/render.js                                    188       24        39      125          0             0.00
demo/helpers.js                                   109        5        11       93          7             7.53
validate.js                                        19        3         0       16          2            12.50
demo/eslint.config.js                              15        1         0       14          0             0.00
stylelint.config.mjs                                4        0         1        3          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
CSS                                       2       256       38         1      217          0             0.00
(ULOC)                                            166
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
css/style.css                                     137       18         0      119          0             0.00
css/tree.css                                      119       20         1       98          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    29      3845      376       203     3266        420           187.59
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      2232
DRYness %                                        0.58
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $93,607
Estimated Schedule Effort (organic) 5.59 months
Estimated People Required (organic) 1.49
Processed 120253 bytes, 0.120 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
