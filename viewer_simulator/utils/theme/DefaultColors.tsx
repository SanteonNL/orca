import { createTheme } from "@mui/material/styles";
import { Plus_Jakarta_Sans } from "next/font/google";
import { Theme } from "@mui/material/styles";

export const plus = Plus_Jakarta_Sans({
  weight: ["300", "400", "500", "600", "700"],
  subsets: ["latin"],
  display: "swap",
  fallback: ["Helvetica", "Arial", "sans-serif"],
});

const darkTheme = createTheme({
  direction: "ltr",
  palette: {
    primary: {
      main: "#115c9e",
      light: "#6ab7ff",
      dark: "#005cb2",
    },
    secondary: {
      main: "#ff9800",
      light: "#ffc947",
      dark: "#c66900",
    },
    success: {
      main: "#66bb6a",
      light: "#98ee99",
      dark: "#338a3e",
      contrastText: "#ffffff",
    },
    info: {
      main: "#29b6f6",
      light: "#73e8ff",
      dark: "#0086c3",
      contrastText: "#ffffff",
    },
    error: {
      main: "#ef5350",
      light: "#ff867c",
      dark: "#b61827",
      contrastText: "#ffffff",
    },
    warning: {
      main: "#ffa726",
      light: "#ffd95b",
      dark: "#c77800",
      contrastText: "#ffffff",
    },
    grey: {
      100: "#2e2e2e",
      200: "#3d3d3d",
      300: "#4b4b4b",
      400: "#616161",
      500: "#757575",
      600: "#9e9e9e",
    },
    text: {
      primary: "#ffffff",
      secondary: "#bdbdbd",
    },
    action: {
      disabledBackground: "rgba(255,255,255,0.12)",
      hoverOpacity: 0.08,
      hover: "#424242",
    },
    divider: "#424242",
    background: {
      default: "#121212",
      paper: "#1e1e1e",
    },
  },
  typography: {
    fontFamily: plus.style.fontFamily,
    h1: {
      fontWeight: 700,
      fontSize: "2.5rem",
      lineHeight: "3rem",
      fontFamily: plus.style.fontFamily,
    },
    h2: {
      fontWeight: 700,
      fontSize: "2rem",
      lineHeight: "2.5rem",
      fontFamily: plus.style.fontFamily,
    },
    h3: {
      fontWeight: 700,
      fontSize: "1.75rem",
      lineHeight: "2rem",
      fontFamily: plus.style.fontFamily,
    },
    h4: {
      fontWeight: 700,
      fontSize: "1.5rem",
      lineHeight: "1.75rem",
    },
    h5: {
      fontWeight: 700,
      fontSize: "1.25rem",
      lineHeight: "1.5rem",
    },
    h6: {
      fontWeight: 700,
      fontSize: "1rem",
      lineHeight: "1.25rem",
    },
    button: {
      textTransform: "capitalize",
      fontWeight: 500,
    },
    body1: {
      fontSize: "1rem",
      fontWeight: 400,
      lineHeight: "1.5rem",
    },
    body2: {
      fontSize: "0.875rem",
      letterSpacing: "0.01rem",
      fontWeight: 400,
      lineHeight: "1.25rem",
    },
    subtitle1: {
      fontSize: "1rem",
      fontWeight: 500,
    },
    subtitle2: {
      fontSize: "0.875rem",
      fontWeight: 500,
    },
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        ".MuiPaper-elevation9, .MuiPopover-root .MuiPaper-elevation": {
          boxShadow: "0 12px 24px rgb(0,0,0,0.15) !important",
        },
        ".rounded-bars .apexcharts-bar-series.apexcharts-plot-series .apexcharts-series path":
        {
          clipPath: "inset(0 0 10% 0 round 10px)",
        },
      },
    },
    MuiCard: {
      styleOverrides: {
        root: {
          borderRadius: "10px",
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          textTransform: "none",
          boxShadow: "none",
          borderRadius: "20px",
        },
        text: {
          padding: "10px 20px",
        },
      },
    },
    MuiSvgIcon: {
      styleOverrides: {
        root: {
          color: "#ffffff",
        },
      },
    },
  },
});

export { darkTheme };
