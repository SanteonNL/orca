import { createTheme } from "@mui/material/styles";

// Create a theme instance.
const theme = createTheme({
  customShadows: {
    z1: '0px 1px 3px rgba(0, 0, 0, 0.12), 0px 1px 2px rgba(0, 0, 0, 0.24)',
    z4: '0px 2px 4px rgba(0, 0, 0, 0.14), 0px 2px 3px rgba(0, 0, 0, 0.20)',
    z8: '0px 5px 8px rgba(0, 0, 0, 0.16), 0px 3px 5px rgba(0, 0, 0, 0.22)',
    z12: '0px 7px 10px rgba(0, 0, 0, 0.18), 0px 4px 6px rgba(0, 0, 0, 0.24)',
    z16: '0px 8px 12px rgba(0, 0, 0, 0.20), 0px 5px 7px rgba(0, 0, 0, 0.26)',
    z20: '0px 10px 14px rgba(0, 0, 0, 0.22), 0px 6px 8px rgba(0, 0, 0, 0.28)',
    z24: '0px 11px 15px rgba(0, 0, 0, 0.24), 0px 7px 9px rgba(0, 0, 0, 0.30)',
    card: '0px 1px 3px rgba(0, 0, 0, 0.12), 0px 1px 2px rgba(0, 0, 0, 0.24)',
    dialog: '0px 5px 15px rgba(0, 0, 0, 0.12)',
    dropdown: '0px 8px 10px rgba(0, 0, 0, 0.14)',
  },
  palette: {
    primary: {
      main: "#FFBD59",
    },
    secondary: {
      main: "#47D7BC",
    },
    error: {
      main: "#fb977d",
    },
  },
});

export default theme;
