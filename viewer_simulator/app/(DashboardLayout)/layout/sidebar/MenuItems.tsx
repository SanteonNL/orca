import {
  IconAperture,
  IconCheckupList,
  IconCopy,
  IconEyeCog,
  IconFlame,
  IconLayoutDashboard,
  IconLogin,
  IconMoodHappy,
  IconTypography,
  IconUserPlus,
} from "@tabler/icons-react";

import { uniqueId } from "lodash";

const Menuitems = [
  {
    navlabel: true,
    subheader: "Home",
  },

  {
    id: uniqueId(),
    title: "Dashboard",
    icon: IconLayoutDashboard,
    href: "/",
  },
  {
    navlabel: true,
    subheader: "Onboarding",
  },
  {
    id: uniqueId(),
    title: "Onboarding Tasks",
    icon: IconCheckupList,
    href: "/onboarding",
  },
  {
    navlabel: true,
    subheader: "View Data",
  },
  {
    id: uniqueId(),
    title: "Data",
    icon: IconCheckupList,
    href: "/bgz",
  },
  {
    navlabel: true,
    subheader: "Auth",
  },
  {
    id: uniqueId(),
    title: "Login",
    icon: IconLogin,
    href: "/authentication/login",
  },
  {
    id: uniqueId(),
    title: "Register",
    icon: IconUserPlus,
    href: "/authentication/register",
  },
];

export default Menuitems;
