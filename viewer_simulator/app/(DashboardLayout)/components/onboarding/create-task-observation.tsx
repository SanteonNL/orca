"use client"
import React, { useState, useEffect } from "react"
import { IconPencilPlus } from "@tabler/icons-react"
import type { Observation, Task, Reference, Identifier } from "fhir/r4"
import Dialog from "@mui/material/Dialog"
import AppBar from "@mui/material/AppBar"
import Toolbar from "@mui/material/Toolbar"
import Typography from "@mui/material/Typography"
import CloseIcon from "@mui/icons-material/Close"
import Slide from "@mui/material/Slide"
import type { TransitionProps } from "@mui/material/transitions"
import {
  Box,
  IconButton,
  TextField,
  MenuItem,
  Select,
  FormControl,
  InputLabel,
  Button,
  Grid,
  FormHelperText,
  Snackbar,
  Alert,
  CircularProgress,
  Chip,
  Portal,
  RadioGroup,
  FormControlLabel,
  Radio,
  Divider,
  Tooltip,
} from "@mui/material"
import { DateTimePicker, LocalizationProvider } from "@mui/x-date-pickers"
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs"
import dayjs from "dayjs"
import "dayjs/locale/nl"
import { getScpContext } from "@/utils/fhirUtils"
import { observationCategories, getObservationCodes, getUnitSuggestions, interpretationOptions } from "./observation-icons"

const Transition = React.forwardRef(function Transition(
  props: TransitionProps & {
    children: React.ReactElement
  },
  ref: React.Ref<unknown>,
) {
  return <Slide direction="up" ref={ref} {...props} />
})

// Observation status options
const statusOptions: Observation["status"][] = [
  "registered",
  "preliminary",
  "final",
  "amended",
  "corrected",
  "cancelled",
  "entered-in-error",
  "unknown",
]

/**
 * Creates a reference if an identifier and/or absolute reference is present for cross-system compatibility.
 * If only a relative reference is present, returns undefined.
 */
function createExternalReference(reference: Reference | undefined): Reference | undefined {
  if (!reference) return undefined

  const externalRef = { ...reference }

  // If there is an absolute reference, return a basic reference
  if (externalRef.reference && !externalRef.reference.startsWith("http")) {
    externalRef.reference = undefined
  }

  // If the reference only contained a relative ref and no identifier, return undefined
  if (!externalRef.reference && !externalRef.identifier?.value) {
    return undefined
  }

  return externalRef
}

/**
 * Extracts a display name from a reference or identifier
 */
function getDisplayFromReference(reference: Reference | undefined): string {
  if (!reference) return "Unknown"

  if (reference.display) {
    return reference.display
  }

  if (reference.identifier?.value) {
    return `${reference.type || "Resource"} (${reference.identifier.system?.split("/").pop() || "ID"}: ${reference.identifier.value})`
  }

  if (reference.reference) {
    return reference.reference
  }

  return "Unknown"
}

export default function CreateTaskObservation({ task, taskFullUrl }: { task: Task; taskFullUrl: string }) {
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [showSuccess, setShowSuccess] = useState(false)
  const [patientDisplay, setPatientDisplay] = useState<string>("")
  const [loading, setLoading] = useState(false)
  const [scpContextIdentifier, setScpContextIdentifier] = useState<Identifier | undefined>()

  // Form state
  const [status, setStatus] = useState<string>("preliminary")
  const [category, setCategory] = useState<string>("vital-signs")
  const [code, setCode] = useState<string>("")
  const [effectiveDateTime, setEffectiveDateTime] = useState<dayjs.Dayjs | null>(dayjs())
  const [valueQuantity, setValueQuantity] = useState<string>("")
  const [valueUnit, setValueUnit] = useState<string>("")
  const [note, setNote] = useState<string>("")
  const [interpretation, setInterpretation] = useState<string>("N") // Default to Normal
  const [referenceRangeLow, setReferenceRangeLow] = useState<string>("")
  const [referenceRangeHigh, setReferenceRangeHigh] = useState<string>("")

  // Get observation codes from shared utility
  const observationCodes = getObservationCodes()

  // Form validation
  const [errors, setErrors] = useState<Record<string, string>>({})

  // Extract patient information and task identifier when component mounts
  useEffect(() => {
    if (task) {
      // Set patient display name
      if (task.for) {
        setPatientDisplay(getDisplayFromReference(task.for))
      }
    }
  }, [task])

  useEffect(() => {
    setScpContextIdentifier(getScpContext(task, taskFullUrl))
  }, [task, taskFullUrl])

  const handleClickOpen = () => {
    setOpen(true)

    // Pre-populate form based on task if needed
    if (task.status === "completed") {
      setStatus("final")
    } else if (task.status === "in-progress") {
      setStatus("preliminary")
    } else {
      setStatus("registered")
    }
  }

  const handleClose = () => {
    setOpen(false)
    resetForm()
  }

  const resetForm = () => {
    setStatus("preliminary")
    setCategory("vital-signs")
    setCode("")
    setEffectiveDateTime(dayjs())
    setValueQuantity("")
    setValueUnit("")
    setNote("")
    setInterpretation("N")
    setReferenceRangeLow("")
    setReferenceRangeHigh("")
    setErrors({})
  }

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {}

    if (!code) newErrors.code = "Observatiecode is verplicht"
    if (!effectiveDateTime) newErrors.effectiveDateTime = "Datum en tijdstip zijn verplicht"
    if (!valueQuantity) newErrors.valueQuantity = "Waarde is verplicht"

    // If reference range is partially filled, ensure both low and high are provided
    if ((referenceRangeLow && !referenceRangeHigh) || (!referenceRangeLow && referenceRangeHigh)) {
      newErrors.referenceRange = "Zowel een ondergrens als bovengrens is nodig voor een referentiewaarde"
    }

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  // Check if the current value is outside the reference range
  const isValueOutsideRange = (): boolean => {
    if (!valueQuantity || (!referenceRangeLow && !referenceRangeHigh)) return false

    const value = Number.parseFloat(valueQuantity)
    const low = referenceRangeLow ? Number.parseFloat(referenceRangeLow) : null
    const high = referenceRangeHigh ? Number.parseFloat(referenceRangeHigh) : null

    if (low !== null && value < low) return true
    if (high !== null && value > high) return true

    return false
  }

  // Suggest interpretation based on value and reference range
  const suggestInterpretation = (): string => {
    if (!valueQuantity || (!referenceRangeLow && !referenceRangeHigh)) return interpretation

    const value = Number.parseFloat(valueQuantity)
    const low = referenceRangeLow ? Number.parseFloat(referenceRangeLow) : null
    const high = referenceRangeHigh ? Number.parseFloat(referenceRangeHigh) : null

    // Critical values (20% beyond reference range)
    if (low !== null && value < low * 0.8) return "LL"
    if (high !== null && value > high * 1.2) return "HH"

    // High/Low values
    if (low !== null && value < low) return "L"
    if (high !== null && value > high) return "H"

    // Within range
    return "N"
  }

  // Update interpretation when value or reference range changes
  useEffect(() => {
    if (valueQuantity && (referenceRangeLow || referenceRangeHigh)) {
      const suggested = suggestInterpretation()
      setInterpretation(suggested)
    }
  }, [valueQuantity, referenceRangeLow, referenceRangeHigh])

  const handleSave = async () => {
    if (!validateForm()) return

    if (!scpContextIdentifier) throw Error("SCP Context Identifier is required - it's undefined")

    setSaving(true)

    try {
      // Create the FHIR Observation resource
      interface ObservationCategory {
        code: string;
        display: string;
        system?: string;
      }

      const selectedCategory: ObservationCategory | undefined = observationCategories.find((c: ObservationCategory) => c.code === category)
      const selectedCode = observationCodes.find((c) => c.code === code)
      const selectedInterpretation = interpretationOptions.find((i) => i.code === interpretation)

      const observation: Observation = {
        resourceType: "Observation",
        status: status as Observation["status"],
        basedOn: [
          {
            type: "CarePlan",
            identifier: scpContextIdentifier,
          },
        ],
        category: [
          {
            coding: [
              {
                system: selectedCategory?.system,
                code: selectedCategory?.code,
                display: selectedCategory?.display,
              },
            ],
          },
        ],
        code: {
          coding: [
            {
              system: selectedCode?.system,
              code: selectedCode?.code,
              display: selectedCode?.display,
            },
          ],
        },
        // Use subject with identifier when possible for cross-system compatibility
        subject: createExternalReference(task.for),
        effectiveDateTime: effectiveDateTime?.toISOString(),
        valueQuantity: {
          value: Number.parseFloat(valueQuantity),
          unit: valueUnit,
          system: "http://unitsofmeasure.org",
          code: valueUnit,
        },
      }

      // Add interpretation if not normal
      if (interpretation !== "N") {
        observation.interpretation = [
          {
            coding: [
              {
                system: "http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation",
                code: selectedInterpretation?.code,
                display: selectedInterpretation?.display,
              },
            ],
            text: selectedInterpretation?.display,
          },
        ]
      }

      // Add reference range if provided
      if (referenceRangeLow || referenceRangeHigh) {
        observation.referenceRange = [
          {
            low: referenceRangeLow
              ? {
                value: Number.parseFloat(referenceRangeLow),
                unit: valueUnit,
                system: "http://unitsofmeasure.org",
                code: valueUnit,
              }
              : undefined,
            high: referenceRangeHigh
              ? {
                value: Number.parseFloat(referenceRangeHigh),
                unit: valueUnit,
                system: "http://unitsofmeasure.org",
                code: valueUnit,
              }
              : undefined,
          },
        ]
      }

      // Add note if provided
      if (note) {
        observation.note = [
          {
            text: note,
          },
        ]
      }

      // Add focus reference if task has one, using identifier when possible
      if (task.focus) {
        const ref = createExternalReference(task.focus)
        if (ref) observation.focus = [ref]
      }

      // Add partOf references if task is part of other tasks
      if (task.partOf && task.partOf.length > 0) {
        observation.partOf = task.partOf
          .map((ref) => createExternalReference(ref))
          .filter((ref) => ref !== undefined) as Reference[]
      }

      // Add context from task's encounter or episode if available
      if (task.encounter) {
        observation.encounter = createExternalReference(task.encounter)
      }

      const resp = await fetch(`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/api/fhir/Observation`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(observation),
      })

      if (!resp.ok) {
        const errorMsg = JSON.stringify(await resp.json(), undefined, 2)
        throw new Error(
          `Failed to save observation: [${resp.status}] ${errorMsg || resp.statusText || "No server message"}`,
        )
      }

      setShowSuccess(true)
      handleClose()
    } catch (error) {
      console.error("Error saving observation:", error)
      if (error instanceof Error) {
        setErrors({ submit: error.message || "Failed to save observation. Please try again." })
      } else {
        setErrors({ submit: "Failed to save observation. Please try again." })
      }
    } finally {
      setSaving(false)
    }
  }

  // Get the color for interpretation display
  const getInterpretationColor = (code: string): string => {
    switch (code) {
      case "N":
        return "success"
      case "A":
        return "error"
      case "H":
      case "L":
        return "warning"
      case "HH":
      case "LL":
        return "error"
      default:
        return "default"
    }
  }

  return (
    <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale="nl">
      <IconButton edge="start" color="inherit" onClick={handleClickOpen} aria-label="register observation">
        <IconPencilPlus />
      </IconButton>

      <Dialog open={open} fullScreen TransitionComponent={Transition}>
        <AppBar sx={{ position: "fixed", backgroundColor: "#121212" }}>
          <Toolbar>
            <IconButton edge="start" color="inherit" onClick={handleClose} aria-label="close" disabled={saving}>
              <CloseIcon />
            </IconButton>
            <Typography sx={{ ml: 2, flex: 1 }} variant="h6" component="div">
              Register New Observation for {patientDisplay}
            </Typography>
            <Button color="inherit" onClick={handleSave} disabled={saving}>
              {saving ? "Saving..." : "Save"}
            </Button>
          </Toolbar>
        </AppBar>

        <Box sx={{ mt: "80px", p: 3 }}>
          {loading ? (
            <Box sx={{ display: "flex", justifyContent: "center", alignItems: "center", height: "50vh" }}>
              <CircularProgress />
            </Box>
          ) : (
            <Grid container spacing={3}>
              {/* Task Information */}
              <Grid item xs={12}>
                <Box sx={{ p: 2, bgcolor: "background.paper", borderRadius: 1, mb: 2 }}>
                  <Typography variant="subtitle2" color="text.secondary">
                    Task ID: {task.id}
                    {scpContextIdentifier && (
                      <Chip
                        color="primary"
                        size="small"
                        label={`SCP Context: ${scpContextIdentifier.system?.split("/").pop()}: ${scpContextIdentifier.value}`}
                        sx={{ ml: 1 }}
                      />
                    )}
                  </Typography>
                  {task.focus && (
                    <Typography variant="subtitle2" color="text.secondary">
                      Focus: {getDisplayFromReference(task.focus)}
                    </Typography>
                  )}
                  <Typography variant="subtitle2" color="text.secondary">
                    Task Status: {task.status}
                  </Typography>
                  {task.for?.identifier && (
                    <Typography variant="subtitle2" color="text.secondary">
                      Patient Identifier: {task.for.identifier.system?.split("/").pop()}: {task.for.identifier.value}
                    </Typography>
                  )}
                </Box>
              </Grid>

              {/* Status */}
              <Grid item xs={12} md={6}>
                <FormControl fullWidth error={!!errors.status}>
                  <InputLabel id="status-label">Status</InputLabel>
                  <Select
                    labelId="status-label"
                    value={status}
                    label="Status"
                    onChange={(e) => setStatus(e.target.value)}
                  >
                    {statusOptions.map((option) => (
                      <MenuItem key={option} value={option}>
                        {option.charAt(0).toUpperCase() + option.slice(1).replace(/-/g, " ")}
                      </MenuItem>
                    ))}
                  </Select>
                  {errors.status && <FormHelperText>{errors.status}</FormHelperText>}
                </FormControl>
              </Grid>

              {/* Category */}
              <Grid item xs={12} md={6}>
                <FormControl fullWidth error={!!errors.category}>
                  <InputLabel id="category-label">Categorie</InputLabel>
                  <Select
                    labelId="category-label"
                    value={category}
                    label="Categorie"
                    onChange={(e) => setCategory(e.target.value)}
                  >
                    {observationCategories.map((category: { code: string; display: string }) => (
                      <MenuItem key={category.code} value={category.code}>
                        {category.display}
                      </MenuItem>
                    ))}
                  </Select>
                  {errors.category && <FormHelperText>{errors.category}</FormHelperText>}
                </FormControl>
              </Grid>

              {/* Observation Code */}
              <Grid item xs={12}>
                <FormControl fullWidth error={!!errors.code}>
                  <InputLabel id="code-label">Observatietype</InputLabel>
                  <Select
                    labelId="code-label"
                    value={code}
                    label="Observation Type"
                    onChange={(e) => {
                      setCode(e.target.value)
                      // Reset unit when code changes
                      setValueUnit("")
                    }}
                  >
                    {observationCodes.map((code: { code: string; display: string }) => (
                      <MenuItem key={code.code} value={code.code}>
                        {code.display}
                      </MenuItem>
                    ))}
                  </Select>
                  {errors.code && <FormHelperText>{errors.code}</FormHelperText>}
                </FormControl>
              </Grid>

              {/* Effective Date Time */}
              <Grid item xs={12}>
                <DateTimePicker
                  maxDate={dayjs()}
                  label="Datum en tijdstip"
                  value={effectiveDateTime}
                  onChange={(newValue) => setEffectiveDateTime(newValue)}
                  slotProps={{
                    textField: {
                      fullWidth: true,
                      error: !!errors.effectiveDateTime,
                      helperText: errors.effectiveDateTime,
                    },
                  }}
                />
              </Grid>

              {/* Value */}
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  label="Waarde"
                  type="number"
                  value={valueQuantity}
                  onChange={(e) => setValueQuantity(e.target.value)}
                  error={!!errors.valueQuantity}
                  helperText={errors.valueQuantity}
                />
              </Grid>

              {/* Unit */}
              <Grid item xs={12} md={6}>
                <FormControl fullWidth error={!!errors.valueUnit}>
                  <InputLabel id="unit-label">Eenheid</InputLabel>
                  <Select
                    labelId="unit-label"
                    value={valueUnit}
                    label="Eenheid"
                    onChange={(e) => setValueUnit(e.target.value)}
                  >
                    {getUnitSuggestions(code).map((unit: string) => (
                      <MenuItem key={unit} value={unit}>
                        {unit}
                      </MenuItem>
                    ))}
                  </Select>
                  {errors.valueUnit && <FormHelperText>{errors.valueUnit}</FormHelperText>}
                </FormControl>
              </Grid>

              {/* Reference Range */}
              <Grid item xs={12}>
                <Typography variant="subtitle1" gutterBottom>
                  Referentiewaarden
                </Typography>
                <Grid container spacing={2}>
                  <Grid item xs={12} sm={6}>
                    <TextField
                      fullWidth
                      label="Ondergrens"
                      type="number"
                      value={referenceRangeLow}
                      onChange={(e) => setReferenceRangeLow(e.target.value)}
                      InputProps={{
                        endAdornment: valueUnit ? (
                          <Typography variant="body2" color="text.secondary">
                            {valueUnit}
                          </Typography>
                        ) : null,
                      }}
                    />
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <TextField
                      fullWidth
                      label="Bovengrens"
                      type="number"
                      value={referenceRangeHigh}
                      onChange={(e) => setReferenceRangeHigh(e.target.value)}
                      InputProps={{
                        endAdornment: valueUnit ? (
                          <Typography variant="body2" color="text.secondary">
                            {valueUnit}
                          </Typography>
                        ) : null,
                      }}
                      error={!!errors.referenceRange}
                      helperText={errors.referenceRange}
                    />
                  </Grid>
                </Grid>
              </Grid>

              {/* Interpretation */}
              <Grid item xs={12}>
                <Divider sx={{ my: 2 }} />
                <Box sx={{ display: "flex", alignItems: "center", mb: 2 }}>
                  <Typography variant="subtitle1" sx={{ mr: 2 }}>
                    Interpretation
                  </Typography>
                  <Chip
                    label={interpretationOptions.find((i) => i.code === interpretation)?.display || "Unknown"}
                    color={getInterpretationColor(interpretation) as any}
                    size="small"
                  />
                  {isValueOutsideRange() && interpretation === "N" && (
                    <Tooltip title="Waarde valt buiten de referentiewaarden maar is gemarkeerd als normaal">
                      <Chip label="Waarschuwing: Waarde valt niet binnen de referentiewaarden" color="warning" size="small" sx={{ ml: 1 }} />
                    </Tooltip>
                  )}
                </Box>
                <FormControl component="fieldset">
                  <RadioGroup row value={interpretation} onChange={(e) => setInterpretation(e.target.value)}>
                    {interpretationOptions.map((option) => (
                      <Tooltip key={option.code} title={option.description}>
                        <FormControlLabel value={option.code} control={<Radio />} label={option.display} />
                      </Tooltip>
                    ))}
                  </RadioGroup>
                </FormControl>
              </Grid>

              {/* Notes */}
              <Grid item xs={12}>
                <TextField
                  fullWidth
                  label="Notities"
                  multiline
                  rows={4}
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                />
              </Grid>

              {/* General error message */}
              {errors.submit && (
                <Grid item xs={12}>
                  <Alert severity="error">{errors.submit}</Alert>
                </Grid>
              )}
            </Grid>
          )}
        </Box>
      </Dialog>

      <Portal>
        <Snackbar
          open={showSuccess}
          autoHideDuration={4000}
          onClose={() => setShowSuccess(false)}
          anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        >
          <Alert onClose={() => setShowSuccess(false)} severity="success">
            Observatie succesvol geregistreerd
          </Alert>
        </Snackbar>
      </Portal>
    </LocalizationProvider>
  )
}