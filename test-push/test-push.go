package main

import (
   "bytes"
   "encoding/gob"
   "flag"
   "fmt"
   "net/http"
   "os"
   "io"
   "io/ioutil"
   "path"
   "mime/multipart"
   "strings"
   "strconv"
)

func die(code int, format string, args ...interface{}) {
   fmt.Fprintf(os.Stderr, format, args...)
   os.Exit(code)
}

func warn(format string, args ...interface{}) {
   fmt.Fprintf(os.Stderr, format, args...)
}

type Test struct {
   ID int64
   StudentGroup string
   Title string
   Temps int16
   Assignatura string
   Especialitat string
   GenDate int64
   NumPermutations int
}

type Permutation struct {
   Index int
   Solutions string
}

func (t *Test) AssignField(key, value string) {
   switch (key) {
   case "Titol": t.Title = value;
   case "Assignatura": t.Assignatura = value;
   case "Especialitat": t.Especialitat = value;
   case "Temps": {
      // TODO: implement more formats
      if (strings.HasSuffix(value, "m")) {
         sminutes := value[:len(value)-len("m")]
         minutes, err := strconv.ParseInt(sminutes, 10, 16)
         if err != nil {
            die(5, "error: cannot parse int64 for 'Temps': '%s'", value)
         }
         t.Temps = int16(minutes)
      } else {
         die(4, "error: 'Temps' format not implemented: '%s'", value)
      }
   }
   case "GenDate": {
      gendate, err := strconv.ParseInt(value, 10, 64)
      if err != nil {
         die(7, "error: 'GenDate' should be an int64 (it's '%s'): %s", value, err)
      }
      t.GenDate = gendate
   }
   case "NumPermutations": {
      numperm, err := strconv.Atoi(value)
      if err != nil {
         die(7, "error: 'NumPermutations' should be an integer: '%s'", value)
      }
      t.NumPermutations = numperm
   }
   default:
      warn("warning: unknown Test field '%s' (value = %s)\n", key, value)
   }
}

func (t *Test) writeToFormFile(writer *multipart.Writer) {
   part, err := writer.CreateFormFile("metadata", "metadata.gob")
   if err != nil {
      die(8, "error: Cannot create 'gob' form file: %s", err)
   }
   err = gob.NewEncoder(part).Encode(t)
   if err != nil {
      die(8, "error: Cannot encode 'gob' data: %s", err)
   }
}

func checkTestMetadata(directory string) {
   // TODO: Use Stat?
   metadataFilename := path.Join(directory, "metadata.csv")
   metadataFile, err := os.Open(metadataFilename)
   if err != nil {
      die(2, "error: Cannot open '%s':\n%s\n", metadataFilename, err)
   }
   metadataFile.Close()
}

func readTestMetadata(directory string) (test Test) {
   metadataFilename := path.Join(directory, "metadata.csv")
   metadataFile, err := os.Open(metadataFilename)
   if err != nil {
      die(2, "error: Cannot open '%s':\n%s\n", metadataFilename, err)
   }

   metadataBytes, err := ioutil.ReadAll(metadataFile)
   if err != nil {
      die(3, "Cannot read '%s':\n%s\n", metadataFilename, err)
   }
   metadataText := string(metadataBytes)

   metadataLines := strings.Split(metadataText, "\n")
   for i := 0; i < len(metadataLines); i++ {
      if len(metadataLines[i]) == 0 {
         continue
      }
      parts := strings.Split(metadataLines[i], ";")
      if len(parts) != 2 {
         die(6, "Metadata has a line with != 2 parts: '%s'\n", metadataLines[i])
      }
      test.AssignField(parts[0], parts[1])
   }
   test.ID = test.GenDate
   return
}

var pdfFilenames = []string{"alln.pdf", "alls.pdf"}

func checkTestBothPdfs(directory string) {
   // TODO: Use Stat?
   for i := range pdfFilenames {
      pdfFilename := path.Join(directory, pdfFilenames[i])
      pdfFile, err := os.Open(pdfFilename)
      if err != nil {
         die(2, "error: Cannot open '%s':\n%s\n", pdfFilename, err)
      }
      pdfFile.Close()
   }
}

func createPdfFormFile(key, pdffilename string, directory string, writer *multipart.Writer) {
   filename := path.Join(directory, pdffilename)
   pdfFile, err := os.Open(filename)
   if err != nil {
      die(2, "Cannot open '%s': %s", filename, err)
   }

   part, err := writer.CreateFormFile(key, pdffilename)
   if err != nil {
      die(8, "error: Cannot create 'pdf' form file: %s", err)
   }

   io.Copy(part, pdfFile)
   pdfFile.Close()
}

func createBothPdfFormFiles(directory string, filenames []string, writer *multipart.Writer) {
   createPdfFormFile("pdf1", filenames[0], directory, writer)
   createPdfFormFile("pdf2", filenames[1], directory, writer)
}

func PushTest(directory string, test Test) {
   var body bytes.Buffer
   formWriter := multipart.NewWriter(&body)
   test.writeToFormFile(formWriter)
   pdffilenames := []string{"alln.pdf", "alls.pdf"}
   createBothPdfFormFiles(directory, pdffilenames, formWriter)
   formWriter.Close()

   url := fmt.Sprintf("http://%s/test", TestServer)
   request, err := http.NewRequest("POST", url, &body)
   if err != nil {
      die(11, "error: Cannot make request: %s\n", err)
   }
   request.Header.Add("Content-Type", formWriter.FormDataContentType())

   client := &http.Client{}
   response, err := client.Do(request)
   if err != nil {
      die(12, "error: Cannot perform http request: %s", err)
   }
   if response.StatusCode == 200 {
      testKey, err := ioutil.ReadAll(response.Body)
      if err != nil {
         die(13, "error: Cannot read response Body")
      }
      fmt.Printf("key = %s\n", string(testKey))
   } else {
      fmt.Printf("Status Code %d: %s\n", response.StatusCode, response.Body)
   }
}

func readPermutations(test Test, directory string) (permutations []Permutation) {
   solutionsFilename := path.Join(directory, "solutions.csv")
   solutionsFile, err := os.Open(solutionsFilename)
   if err != nil {
      die(2, "error: Cannot open '%s':\n%s\n", solutionsFilename, err)
   }

   solutionsBytes, err := ioutil.ReadAll(solutionsFile)
   if err != nil {
      die(3, "Cannot read '%s':\n%s\n", solutionsFilename, err)
   }
   solutionsText := string(solutionsBytes)

   solutionsLines := strings.Split(solutionsText, "\n")
   for i := 0; i < len(solutionsLines); i++ {
      if len(solutionsLines[i]) == 0 {
         continue
      }
      parts := strings.Split(solutionsLines[i], ";")
      if len(parts) != 2 {
         die(6, "Metadata has a line with != 2 parts: '%s'\n", solutionsLines[i])
      }
      if fmt.Sprintf("%d", i) != parts[0] {
         die(7, "Expected index '%d' in solutions.csv (got '%s')", i, parts[0])
      }
      perm := Permutation{Index: i, Solutions: parts[1]}
      permutations = append(permutations, perm)
   }
   return permutations
}


func PushPermutation(directory string, test Test, perm Permutation) {
   var body bytes.Buffer
   formWriter := multipart.NewWriter(&body)

   // write pdf files
   pdffilenames := []string{"", ""}
   for i, suffix := range []string{"n", "s"} {
      pdffilenames[i] = fmt.Sprintf("%04d%s.pdf", perm.Index, suffix)
   }
   createBothPdfFormFiles(directory, pdffilenames, formWriter)

   // write gob encoding of struct
   part, err := formWriter.CreateFormFile("permutation", "permutation.gob")
   if err != nil {
      die(8, "error: Cannot create 'gob' form file: %s", err)
   }
   err = gob.NewEncoder(part).Encode(perm)
   if err != nil {
      die(8, "error: Cannot encode 'gob' data: %s", err)
   }

   // close form (IMPORTANT!)
   formWriter.Close()

   // Make request
   url := fmt.Sprintf("http://%s/test/%d/permutation", TestServer, test.ID)
   request, err := http.NewRequest("POST", url, &body)
   if err != nil {
      die(11, "error: Cannot make request: %s\n", err)
   }
   request.Header.Add("Content-Type", formWriter.FormDataContentType())

   // use an http client to send the request
   client := &http.Client{}
   response, err := client.Do(request)
   if err != nil {
      die(12, "error: Cannot perform http request: %s", err)
   }
   if response.StatusCode == 200 {
      testKey, err := ioutil.ReadAll(response.Body)
      if err != nil {
         die(13, "error: Cannot read response Body")
      }
      fmt.Printf("key = %s\n", string(testKey))
   } else {
      fmt.Printf("Status Code %d: %s\n", response.StatusCode, response.Body)
   }


}

// const TestServer = "127.0.0.1:8080"
const TestServer = "test-server-161210.appspot.com"

func main() {
   flag.Parse()
   args := flag.Args()

   if len(args) != 1 {
      die(1, "usage: test-push <test-dir>\n")
   }

   directory := args[0]
   checkTestMetadata(directory)
   checkTestBothPdfs(directory)

   test := readTestMetadata(directory)
   PushTest(directory, test)

   permutations := readPermutations(test, directory)
   for i := 0; i < len(permutations); i++ {
      PushPermutation(directory, test, permutations[i])
   }
}
