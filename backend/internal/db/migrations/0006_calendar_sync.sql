-- Marca los vencimientos retirados automáticamente al recalcular el calendario
-- por un cambio de perfil o de fecha de inicio de actividad. Un cambio manual
-- de estado limpia la marca para no reabrir decisiones de la persona usuaria.
ALTER TABLE filings ADD COLUMN auto_hidden INTEGER NOT NULL DEFAULT 0;
